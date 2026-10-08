import axios from 'axios';
import { defineStore } from 'pinia';
import { computed, markRaw, onUnmounted, ref, watch } from 'vue';
import { RouteLocationNormalizedLoaded } from 'vue-router';

export type TrustModelInstance = {
  id: string,
  fullTMI: string,
  client: string,
  sessionID: string,
  template: string,
  active: boolean,
  latestVersion: number,
  // number of versions whose ATL result set contains each trust decision
  decisionCounts: DecisionCounts,
  // trust decisions per version (index = version, bit 1 << TdResults value per decision, 0 without ATLs), only for
  // watched TMIs whose page fetched them
  decisionsByVersion?: number[],
  states: {[key: string]: TrustModelInstanceState},
  atls: {[key: string]: ActualTrustworthinessLevel},
  updates: {[key: string]: TrustModelInstanceUpdate}
};

export type TrustModelInstanceUpdate = any

export type TrustDecision = 'TRUSTWORTHY' | 'NOT_TRUSTWORTHY' | 'UNDECIDABLE';

export type DecisionCounts = {[decision in TrustDecision]: number};

// names of the trust decisions by their value in TdResults (core.TrustDecision)
export const TRUST_DECISIONS: TrustDecision[] = ['NOT_TRUSTWORTHY', 'TRUSTWORTHY', 'UNDECIDABLE'];

export const TRUST_DECISION_LABELS: {[decision in TrustDecision]: string} = {
  TRUSTWORTHY: 'Trustworthy',
  NOT_TRUSTWORTHY: 'Not trustworthy',
  UNDECIDABLE: 'Undecidable'
};

// CSS colors of the trust decisions, shared by the version timeline and the decision summary of the TMI overview; grey
// is no theme color, so it is derived from the text color
export const TRUST_DECISION_CSS_COLORS: {[decision in TrustDecision]: string} = {
  TRUSTWORTHY: 'rgb(var(--v-theme-success))',
  NOT_TRUSTWORTHY: 'rgb(var(--v-theme-error))',
  UNDECIDABLE: 'rgba(var(--v-theme-on-surface), 0.38)'
};

export const TRUST_DECISION_COLORS: {[decision in TrustDecision]: string} = {
  TRUSTWORTHY: 'success',
  NOT_TRUSTWORTHY: 'error',
  UNDECIDABLE: 'grey'
};

function emptyDecisionCounts(): DecisionCounts {
  return { TRUSTWORTHY: 0, NOT_TRUSTWORTHY: 0, UNDECIDABLE: 0 };
}

// trust decisions of the latest ATL result set counted live per TMI, so that a result set replacing it for the same
// version replaces its decisions instead of counting them twice; not part of the reactive state
const lastCountedDecisions = new Map<string, { version: number, decisions: Set<TrustDecision> }>();

export type SubjectiveLogicOpinion = {
  belief: number,
  disbelief: number,
  uncertainty: number,
  base_rate: number
};

export type ActualTrustworthinessLevel = {
  TmiID: string,
  Version: number,
  SlResults: {[key: string]: SubjectiveLogicOpinion},
  PpResults: {[key: string]: number},
  TdResults: {[key: string]: number}
};

export type TrustModelInstanceState = {
  Version: number,
  Fingerprint: number,
  Structure: {
    operator: string,
    adjacency_list: {
      sourceNode: string,
      targetNodes: string[]
    }[]
  },
  Values: {
    [key: string]: {
      source: string,
      destination: string,
      opinion: SubjectiveLogicOpinion
    }[]
  },
  RTLs: { [key: string]: SubjectiveLogicOpinion }
};

export type Session = {
  SessionID: string,
  Client: string,
  IsActive: boolean,
  Template: string,
  TMIs: string[],
};

export const useAppStore = defineStore('app', {
  state: () => ({
    loading: false as (boolean),
    socket: null as (WebSocket|null),
    sessions: {} as {[key: string]: Session},
    trustModelInstances: {} as {[key: string]: TrustModelInstance},
    // number of pages currently showing a TMI; states, updates, and ATLs are only kept for watched TMIs
    watchedTrustModelInstances: {} as {[key: string]: number},
    // watched TMIs for which newly arriving versions are added live, with the maximum amount of versions to keep
    liveHistoryLimits: {} as {[key: string]: number},
    // incremented whenever the state has been re-fetched after a reconnect, so that pages can re-fetch their data
    syncGeneration: 0,
  }),

  actions: {
    setSocket(socket: WebSocket|null) {
      this.socket = socket;
    },

    watchTrustModelInstance(key: string) {
      this.watchedTrustModelInstances[key] = (this.watchedTrustModelInstances[key] || 0) + 1;
    },

    unwatchTrustModelInstance(key: string) {
      this.watchedTrustModelInstances[key] = (this.watchedTrustModelInstances[key] || 0) - 1;
      if (this.watchedTrustModelInstances[key] <= 0) {
        delete this.watchedTrustModelInstances[key];
        delete this.liveHistoryLimits[key];
        // free the history of TMIs that are not shown anymore, it can be fetched again from the server
        const tmi = this.trustModelInstances[key];
        if (tmi) {
          tmi.states = {};
          tmi.updates = {};
          tmi.atls = {};
          delete tmi.decisionsByVersion;
        }
      }
    },

    isWatched(key: string): boolean {
      return (this.watchedTrustModelInstances[key] || 0) > 0;
    },

    /*
      setLiveHistoryLimit makes newly arriving versions of a watched TMI be added, keeping at most limit versions;
      undefined stops adding them.
    */
    setLiveHistoryLimit(key: string, limit: number | undefined) {
      if (limit === undefined) {
        delete this.liveHistoryLimits[key];
        return;
      }
      this.liveHistoryLimits[key] = limit;
      this.trimHistory(key);
    },

    clearTrustModelInstanceHistory(key: string) {
      const tmi = this.trustModelInstances[key];
      if (tmi) {
        tmi.states = {};
        tmi.updates = {};
        tmi.atls = {};
      }
    },

    /*
      retainTrustModelInstanceVersions removes all versions of a TMI except the given ones.
    */
    retainTrustModelInstanceVersions(key: string, versions: number[]) {
      const tmi = this.trustModelInstances[key];
      if (!tmi) {
        return;
      }
      for (const version of Object.keys(tmi.states).map(Number).filter(v => !versions.includes(v))) {
        delete tmi.states[version];
        delete tmi.updates[version];
        delete tmi.atls[version];
      }
    },

    isLive(key: string): boolean {
      return this.isWatched(key) && this.liveHistoryLimits[key] !== undefined;
    },

    /*
      trimHistory removes the oldest versions of a live TMI exceeding its history limit.
    */
    trimHistory(key: string) {
      const tmi = this.trustModelInstances[key];
      const limit = this.liveHistoryLimits[key];
      if (!tmi || limit === undefined) {
        return;
      }
      const versions = Object.keys(tmi.states).map(Number).sort((a, b) => a - b);
      for (const version of versions.slice(0, Math.max(0, versions.length - limit))) {
        delete tmi.states[version];
        delete tmi.updates[version];
        delete tmi.atls[version];
      }
    },

    processMessages(msgs: any[]) {
      // applying all messages of a batch within one action results in a single re-rendering
      for (const msg of msgs) {
        this.processMessage(msg);
      }
    },

    processMessage(msg: any) {
      // process events by the taf and update state by replicating the go logic
      // states, updates, and ATLs are only kept for watched TMIs and are not made reactive, as they never change

      switch (msg.EventType) {
        case 'SESSION_CREATED':
          /*
            func (s *State) handleSessionCreatedEvent(event listener.SessionCreatedEvent) {
              s.sessions[event.SessionID] = &sessionState{
                Client:   event.ClientID,
                IsActive: true,
                TMIs:     make([]string, 0),
                Template: event.TrustModelTemplate,
              }
            }
          */
          if (msg.SessionID) {
            this.sessions[msg.SessionID] = {
              SessionID: msg.SessionID,
              Template: msg.TrustModelTemplate,
              Client: msg.ClientID,
              IsActive: true,
              TMIs: [],
            };
          }
          break;

        case 'SESSION_TORNDOWN':
          /*
            func (s *State) handleSessionTorndownEvent(event listener.SessionTorndownEvent) {
              if _, exists := s.sessions[event.SessionID]; exists {
                s.sessions[event.SessionID].IsActive = false
              }
            }
          */
          if (msg.SessionID && this.sessions[msg.SessionID]) {
            this.sessions[msg.SessionID].IsActive = false;
          }
          break;

        case 'ATL_REMOVED':
          // ignore
          break;

        case 'TRUST_MODEL_INSTANCE_SPAWNED': {
          /*
            func (s *State) handleTMISpawned(event listener.TrustModelInstanceSpawnedEvent) {
              fullTMI := event.FullTMI

              _, sessionID, _, _ := core.SplitFullTMIIDentifier(fullTMI)
              if _, exists := s.sessions[sessionID]; exists {
                s.sessions[sessionID].TMIs = append(s.sessions[sessionID].TMIs, fullTMI)
              }

              s.tmis[fullTMI] = &tmiMetaState{
                IsActive:      true,
                LatestVersion: 0,
                Update:        make(map[int][]core.Update),
                States:        make(map[int]tmiState),
                Template:      event.Template,
                ID:            event.ID,
                FullTMI:       event.FullTMI,
                ATLs:          make(map[int]core.AtlResultSet),
              }
              s.tmis[fullTMI].States[event.Version] = tmiState{
                Version:     event.Version,
                Fingerprint: event.Fingerprint,
                Structure:   event.Structure,
                Values:      event.Values,
                RTLs:        event.RTLs,
              }

            }
          */
          const parts = msg.FullTMI.split('/');

          if (this.sessions[parts[3]]) {
            this.sessions[parts[3]].TMIs.push(parts[5]);
          }

          this.trustModelInstances[msg.FullTMI] = {
            id: msg.ID,
            fullTMI: msg.FullTMI,
            client: parts[2],
            sessionID: parts[3],
            template: parts[4],
            active: true,
            latestVersion: msg.Version,
            decisionCounts: emptyDecisionCounts(),
            updates: {},
            states: this.isLive(msg.FullTMI) ? {
              [String(msg.Version)]: markRaw({
                Version: msg.Version,
                Fingerprint: msg.Fingerprint,
                Structure: msg.Structure,
                Values: msg.Values,
                RTLs: msg.RTLs
              })
            } : {},
            atls: {}
          };
          break;
        }

        case 'ATL_UPDATED': {
          /*
            func (s *State) handleATLUpdatedEvent(event listener.ATLUpdatedEvent) {
              fullTMI := event.FullTMI
              _, exists := s.tmis[fullTMI]
              if !exists {
                return
              } else {
                s.logger.Warn(fmt.Sprintf("%+v", event.NewATLs))
              }
              s.tmis[fullTMI].ATLs[event.NewATLs.Version()] = event.NewATLs
            }
          */
          const key = msg.FullTMI;
          if (this.trustModelInstances[key]) {
            // counted for every TMI, as the overview shows the counts of all of them
            const decisions = new Set(Object.values(msg.NewATLs.TdResults ?? {}).map((d) => TRUST_DECISIONS[d as number]));
            const counts = this.trustModelInstances[key].decisionCounts;
            const last = lastCountedDecisions.get(key);
            if (last && last.version === msg.NewATLs.Version) {
              last.decisions.forEach((d) => counts[d]--);
            }
            decisions.forEach((d) => counts[d]++);
            lastCountedDecisions.set(key, { version: msg.NewATLs.Version, decisions });

            const byVersion = this.trustModelInstances[key].decisionsByVersion;
            if (byVersion && this.isWatched(key)) {
              byVersion[msg.NewATLs.Version] = [...decisions].reduce((bits, d) => bits | (1 << TRUST_DECISIONS.indexOf(d)), 0);
            }
          }
          if (this.trustModelInstances[key] && this.isLive(key) && this.trustModelInstances[key].states[msg.NewATLs.Version]) {
            if (!this.trustModelInstances[key].atls) {
              this.trustModelInstances[key].atls = {};
            }
            this.trustModelInstances[key].atls[msg.NewATLs.Version] = markRaw(msg.NewATLs);
          }
          break;
        }

        case 'TRUST_MODEL_INSTANCE_DELETED': {
          /*
            func (s *State) handleTMIDeleted(event listener.TrustModelInstanceDeletedEvent) {
              fullTMI := event.FullTMI
              s.tmis[fullTMI].IsActive = false
            }
          */

          const key = msg.FullTMI;
          if (this.trustModelInstances[key]) {
            this.trustModelInstances[key].active = false;
          }
          break;
        }

        case 'TRUST_MODEL_INSTANCE_UPDATED': {
          /*
            func (s *State) handleTMIUpdated(event listener.TrustModelInstanceUpdatedEvent) {
              fullTMI := event.FullTMI
              // If there exists already an entry for that version, this means we have received another update that yields the same
              // version number. This means that the second update has failed to increase the version number and can be ignored.
              _, exists := s.tmis[fullTMI].States[event.Version]
              if exists {
                s.tmis[fullTMI].Update[event.Version+1] = []core.Update{event.Update}
                return
              }
              s.tmis[fullTMI].States[event.Version] = tmiState{
                Version:     event.Version,
                Fingerprint: event.Fingerprint,
                Structure:   event.Structure,
                Values:      event.Values,
                RTLs:        event.RTLs,
              }
              if s.tmis[fullTMI].Update[event.Version] == nil {
                s.tmis[fullTMI].Update[event.Version] = make([]core.Update, 0)
              }
              s.tmis[fullTMI].Update[event.Version] = append(s.tmis[fullTMI].Update[event.Version], event.Update)
              s.tmis[fullTMI].LatestVersion = event.Version
            }
          */

          const key = msg.FullTMI;
          if (this.trustModelInstances[key]) {
            this.trustModelInstances[key].latestVersion = Math.max(this.trustModelInstances[key].latestVersion, msg.Version);
            if (!this.isLive(key)) {
              break;
            }

            if (!this.trustModelInstances[key].states) {
              this.trustModelInstances[key].states = {};
            }

            if (msg.Update) {
              if (!this.trustModelInstances[key].updates) {
                this.trustModelInstances[key].updates = {};
              }

              if (this.trustModelInstances[key].states[msg.Version]) {
                if (!Array.isArray(this.trustModelInstances[key].updates[msg.Version + 1])) {
                  this.trustModelInstances[key].updates[msg.Version + 1] = [];
                }

                this.trustModelInstances[key].updates[msg.Version + 1].push(markRaw(msg.Update));
                return;
              }

              if (!Array.isArray(this.trustModelInstances[key].updates[msg.Version])) {
                this.trustModelInstances[key].updates[msg.Version] = [];
              }

              this.trustModelInstances[key].updates[msg.Version].push(markRaw(msg.Update));
            }

            this.trustModelInstances[key].states[msg.Version] = markRaw(msg);
            this.trimHistory(key);
          }
          break;
        }
      }
    },

    async fetchTrustModelInstance(client: string, sessionID: string, template: string, id: string, version: string='all') {
      const res = await axios.get(`/api/tmis/${client}/${sessionID}/${template}/${id}/${version}`);
      const key = res.data.fullTMI as string;

      if (!this.trustModelInstances[key]) {
        const parts = res.data.fullTMI.split('/');
        this.trustModelInstances[key] = {
          id: res.data.id,
          fullTMI: res.data.fullTMI,
          client: parts[2],
          sessionID: parts[3],
          template: res.data.template,
          active: res.data.active,
          latestVersion: res.data.latestVersion,
          decisionCounts: emptyDecisionCounts(),
          atls: {},
          states: {},
          updates: {},
        };
      }

      if (res.data.atls?.Version !== undefined) {
        this.trustModelInstances[key].atls[res.data.atls.Version] = markRaw(res.data.atls);
      }

      if (res.data.state?.Version !== undefined) {
        this.trustModelInstances[key].states[res.data.state.Version] = markRaw(res.data.state);

        if (Array.isArray(res.data.updates)) {
          this.trustModelInstances[key].updates[res.data.state.Version] = res.data.updates.map(markRaw);
        }
      } else if (typeof(res.data.states) === 'object') {
        this.trustModelInstances[key].states = rawValues(res.data.states);
        if (typeof(res.data.updates) === 'object') {
          this.trustModelInstances[key].updates = Object.fromEntries(
            Object.entries(res.data.updates || {}).map(([k, v]) => [k, Array.isArray(v) ? v.map(markRaw) : v])
          );
        }
        if (typeof(res.data.atls) === 'object') {
          this.trustModelInstances[key].atls = rawValues(res.data.atls);
        }
      }
    },

    /*
      fetchTrustModelInstanceVersions fetches a page of versions (state, updates, and ATLs) of a TMI in descending order,
      optionally only versions older than before and only versions matching search. Returns the amount of fetched
      versions and the total amount of matching versions. The result is discarded if isCurrent returns false, e.g.,
      because a newer search has been started in the meantime.
    */
    async fetchTrustModelInstanceVersions(client: string, sessionID: string, template: string, id: string, limit: number, before?: number, search?: string, decision?: string, isCurrent: () => boolean = () => true): Promise<{ fetched: number, matches: number }> {
      const params: {[key: string]: string | number} = { limit };
      if (before !== undefined) {
        params.before = before;
      }
      if (search) {
        params.search = search;
      }
      // only versions whose ATLs contain this trust decision, resolved by the server over the whole history
      if (decision) {
        params.decision = decision;
      }
      const res = await axios.get(`/api/tmis/${client}/${sessionID}/${template}/${id}/versions`, { params });
      if (!isCurrent()) {
        return { fetched: 0, matches: 0 };
      }
      const key = res.data.fullTMI as string;
      const parts = key.split('/');

      if (!this.trustModelInstances[key]) {
        this.trustModelInstances[key] = {
          id: res.data.id,
          fullTMI: key,
          client: parts[2],
          sessionID: parts[3],
          template: res.data.template,
          active: res.data.active,
          latestVersion: res.data.latestVersion,
          decisionCounts: emptyDecisionCounts(),
          atls: {},
          states: {},
          updates: {},
        };
      }

      const tmi = this.trustModelInstances[key];
      tmi.latestVersion = Math.max(tmi.latestVersion, res.data.latestVersion);
      for (const item of res.data.versions) {
        tmi.states[item.version] = markRaw(item.state);
        tmi.updates[item.version] = Array.isArray(item.updates) ? item.updates.map(markRaw) : item.updates;
        if (item.atls) {
          tmi.atls[item.version] = markRaw(item.atls);
        }
      }
      return { fetched: res.data.versions.length, matches: res.data.matches };
    },

    /*
      fetchTrustModelInstanceDecisions fetches the trust decisions of all versions of a known TMI, which are then kept up
      to date while it is watched.
    */
    async fetchTrustModelInstanceDecisions(client: string, sessionID: string, template: string, id: string) {
      const res = await axios.get(`/api/tmis/${client}/${sessionID}/${template}/${id}/decisions`);
      const tmi = this.trustModelInstances[`//${client}/${sessionID}/${template}/${id}`];
      if (tmi) {
        tmi.decisionsByVersion = res.data.decisions;
        tmi.latestVersion = Math.max(tmi.latestVersion, res.data.latestVersion);
      }
    },

    async fetchTrustModelInstances() {
      const req = await axios.get('/api/tmis');
      // update metadata only, so that already fetched states of watched TMIs are kept
      for (const [key, entry] of Object.entries(req.data) as [string, any][]) {
        const parts = key.split('/');
        const existing = this.trustModelInstances[entry.fullTMI];

        this.trustModelInstances[entry.fullTMI] = {
          id: entry.id,
          fullTMI: entry.fullTMI,
          client: parts[2],
          sessionID: parts[3],
          template: entry.template,
          active: entry.active,
          latestVersion: entry.latestVersion,
          decisionCounts: entry.decisionCounts ?? emptyDecisionCounts(),
          states: existing?.states || {},
          updates: existing?.updates || {},
          atls: existing?.atls || {}
        };
      }
    },

    async fetchSessions() {
      const req = await axios.get('/api/sessions');
      this.sessions = Object.fromEntries(
        Object.entries(req.data).map(([key, entry]: [string, any]) => {
          return [key, {
            SessionID: key,
            Client: entry.Client,
            IsActive: entry.IsActive,
            TMIs: (Array.isArray(entry.TMIs) ? entry.TMIs : []).map((e: string) => e.split('/').pop()),
            Template: entry.Template,
          }];
        })
      );
    },

    async init() {
      await this.fetchTrustModelInstances();
      await this.fetchSessions();
    },

    /*
      resync re-fetches the state after a reconnect of the web socket, as events may have been missed in the meantime.
    */
    async resync() {
      await this.init();
      this.syncGeneration++;
    }
  }
})

function rawValues<T extends object>(obj: {[key: string]: T}): {[key: string]: T} {
  return Object.fromEntries(Object.entries(obj || {}).map(([k, v]) => [k, v && typeof v === 'object' ? markRaw(v) : v]));
}

/*
useWatchedTrustModelInstance marks the TMI of the current route as watched while the calling component is mounted, so
that its states, updates, and ATLs are kept in the store, and calls refresh whenever the data needs to be (re-)fetched.
If liveHistoryLimit is given, newly arriving versions are added to the store, keeping at most that many versions
(none while it returns undefined).
*/
export function useWatchedTrustModelInstance(route: RouteLocationNormalizedLoaded, refresh: () => unknown, liveHistoryLimit?: () => number | undefined) {
  const store = useAppStore();
  // the TMI currently registered as watched; tracked separately from the route, as the route already points to the
  // next page when this component is unmounted
  const watchedKey = ref<string | null>(null);
  const key = computed(() => `//${route.params.client}/${route.params.sessionID}/${route.params.template}/${route.params.id}`);

  watch(key, (newKey) => {
    if (route.params.id === undefined || newKey === watchedKey.value) {
      // the route does not point to a TMI anymore, e.g., while navigating away
      return;
    }
    if (watchedKey.value) {
      store.unwatchTrustModelInstance(watchedKey.value);
    }
    watchedKey.value = newKey;
    store.watchTrustModelInstance(newKey);
    if (liveHistoryLimit) {
      store.setLiveHistoryLimit(newKey, liveHistoryLimit());
    }
    refresh();
  }, { immediate: true });
  watch(() => store.syncGeneration, () => refresh());
  if (liveHistoryLimit) {
    watch(liveHistoryLimit, (limit) => watchedKey.value && store.setLiveHistoryLimit(watchedKey.value, limit));
  }
  onUnmounted(() => {
    if (watchedKey.value) {
      store.unwatchTrustModelInstance(watchedKey.value);
    }
  });

  return computed(() => watchedKey.value ? store.trustModelInstances[watchedKey.value] : undefined);
}
