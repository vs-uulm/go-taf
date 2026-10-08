<template>
  <Teleport to="#toolbar">
    <faceted-search v-model="filteredItems" :columns="searchColumns" :items="items" sync-to-query v-model:sort-by="sortBy" :defaults="{ sort: '-version' }" remote-search @update:search="onSearch" @update:remoteFilters="onRemoteFilters" />

    <v-chip v-if="searchTerm || decision" size="small" class="mx-1 align-self-center" label :title="`${items.length} of ${matches} versions matching the search are loaded`">{{ items.length }} / {{ matches }} matching versions</v-chip>
    <v-chip v-else size="small" class="mx-1 align-self-center" label :title="`The latest ${items.length} versions are shown`">{{ items.length }} versions</v-chip>
    <v-btn icon :title="`Load ${PAGE_SIZE} older versions`" size="small" @click="loadOlder" :disabled="!hasOlder" variant="text">
      <v-icon icon="mdi-history" />
    </v-btn>

    <v-btn icon title="Refresh" size="small" @click="refresh" variant="text">
      <v-icon icon="mdi-reload" />
    </v-btn>
  </Teleport>

  <!--
    item-height estimates the row height (rows are at least as high as the graph), as the virtual table otherwise
    renders all rows if the items arrive before any row has been measured; item-value keeps rows stable when new
    versions are added at the top
  -->
  <!-- scroll does not bubble, so it is caught in the capture phase from the table's scroll container -->
  <v-data-table-virtual :headers="headers" :items="filteredItems" item-value="version" :item-height="ROW_HEIGHT" :height="height" v-resize="onResize" v-model:sort-by="sortBy" @scroll.capture="onScroll">
    <template #[`item.version`]="{ item }">
      <v-chip class="pr-0 mt-1 text-no-wrap" :to="`/tmis/${route.params.client as string}/${route.params.sessionID as string}/${route.params.template as string}/${route.params.id as string}/${item.version}`">
        Version
        <v-chip class="ml-1 font-weight-bold">{{ item.version }}</v-chip>
      </v-chip>
    </template>
    <template #[`item.updates`]="{ item }">
      <!-- spacing is set explicitly, as Vuetify 4 no longer resets the browser's default margins and paddings -->
      <ul class="mt-1 pa-0">
        <li v-for="(update, i) in item.updates" :key="i"><pre class="ma-0">{{ update }}</pre></li>
      </ul>
    </template>
    <template #[`item.atls`]="{ item }">
      <v-card variant="outlined" class="mx-2 my-1" v-for="(_, scope) in item.atls?.SlResults" :key="scope">
        <template #title>
          <div class="text-label-medium">ATL Scope: <code>{{ scope }}</code></div>
        </template>

        <v-table density="compact" class="mt-n4">
          <tbody>
            <tr>
              <th class="text-left">Subjective Logic</th>
              <td v-if="item.atls.SlResults[scope]?.belief !== undefined"><code>({{ item.atls.SlResults[scope].belief?.toFixed(2) }} / {{ item.atls.SlResults[scope].disbelief.toFixed(2) }} / {{ item.atls.SlResults[scope].uncertainty.toFixed(2) }} / {{ item.atls.SlResults[scope].base_rate.toFixed(2) }})</code></td>
              <td v-else><code>{{ item.atls.SlResults[scope] }}</code></td>
            </tr>
            <tr>
              <th class="text-left">Projected Probability</th>
              <td><code>{{ item.atls.PpResults[scope] }}</code></td>
            </tr>
            <tr>
              <th class="text-left">Trust Decision</th>
              <td>
                <code>{{ item.atls.TdResults[scope] }}</code>
                <v-chip v-if="TRUST_DECISIONS[item.atls.TdResults[scope]]" size="x-small" label class="ml-2" :color="TRUST_DECISION_COLORS[TRUST_DECISIONS[item.atls.TdResults[scope]]]" variant="flat">{{ TRUST_DECISION_LABELS[TRUST_DECISIONS[item.atls.TdResults[scope]]] }}</v-chip>
              </td>
            </tr>
          </tbody>
        </v-table>
      </v-card>
    </template>
    <template #[`item.state`]="{ item }">
      <v-card variant="outlined" class="mx-2 my-1" v-for="(rows, scope) in item.state.Values" :key="scope">
        <template #title>
          <div class="text-label-medium">Scope: <code>{{ scope }}</code></div>
        </template>

        <v-table density="compact" class="mt-n4">
          <thead>
            <tr>
              <th class="text-left">Source</th>
              <th class="text-left">Destination</th>
              <th class="text-left">Opinion</th>
              <!--
              <th class="text-left">Belief</th>
              <th class="text-left">Disbelief</th>
              <th class="text-left">Uncertainty</th>
              <th class="text-left">Base Rate</th>
              -->
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, i) in rows" :key="i">
              <td>{{ row.source }}</td>
              <td>{{ row.destination }}</td>
              <td><code>({{ row.opinion.belief.toFixed(2) }} / {{ row.opinion.disbelief.toFixed(2) }} / {{ row.opinion.uncertainty.toFixed(2) }} / {{ row.opinion.base_rate.toFixed(2) }})</code></td>
              <!--
              <td><code>{{ row.opinion.belief.toFixed(2) }}</code></td>
              <td><code>{{ row.opinion.disbelief.toFixed(2) }}</code></td>
              <td><code>{{ row.opinion.uncertainty.toFixed(2) }}</code></td>
              <td><code>{{ row.opinion.base_rate.toFixed(2) }}</code></td>
              -->
            </tr>
          </tbody>
        </v-table>
      </v-card>
    </template>
    <template #[`item.graph`]="{ item }">
      <div style="min-height: 500px; min-width: 500px; height: 100%;">
        <trust-graph :state="item.state" :zoom-enabled="false" :pan-enabled="false" :zoom-level="2.5" />
      </div>
    </template>
  </v-data-table-virtual>
</template>

<style>
.v-data-table__tr {
  vertical-align: top;
}
</style>

<script lang='ts' setup>
import { computed, nextTick, ref } from 'vue';
import { useRoute } from 'vue-router';

import { TRUST_DECISION_COLORS, TRUST_DECISION_LABELS, TRUST_DECISIONS, useAppStore, useWatchedTrustModelInstance } from '@/stores/app';
import { Column, SortItem } from '@/types';
import router from '@/router';

const filteredItems = ref<any[]>([]);
const sortBy = ref<SortItem[]>([]);

const route = useRoute();

const store = useAppStore();
const headers: Column[] = [{
  // wide enough for the "Version" chip with a number of several digits, which a maximum width cut off
  minWidth: '160px',
  filterable: true,
  title: 'Version',
  key: 'version',
}, {
  filterable: true,
  maxWidth: '300px',
  title: 'Updates',
  key: 'updates'
}, {
  filterable: false,
  title: 'Graph',
  key: 'graph'
}, {
  filterable: false,
  maxWidth: '350px',
  title: 'State',
  key: 'state'
}, {
  maxWidth: '350px',
  filterable: false,
  title: 'ATLs',
  key: 'atls'
}];

// the search additionally offers a filter on the trust decision, which the server resolves over the whole history
const searchColumns: Column[] = [...headers, {
  filterable: true,
  remoteFilter: true,
  title: 'Trust Decision',
  key: 'decision',
  filterSettings: {
    type: 'select',
    items: Object.entries(TRUST_DECISION_LABELS).map(([value, title]) => ({ title, value, color: TRUST_DECISION_COLORS[value as keyof typeof TRUST_DECISION_COLORS] })),
    itemText: 'title',
    itemValue: 'value'
  }
}];

// versions are loaded page by page, newly arriving versions are added while keeping at most historyLimit versions
const PAGE_SIZE = 100;
// estimated row height in pixels, see the comment on the table
const ROW_HEIGHT = 500;
const historyLimit = ref<number | undefined>(PAGE_SIZE);
// whether the last fetch of a search or filter returned a full page, i.e. more matching versions may exist
const fetchedHasOlder = ref(false);
// oldest version below which the server returned no versions, to stop loading if versions are missing
const exhaustedBelow = ref<number | undefined>(undefined);
// the search term and the trust decision filter are resolved by the server, so that they cover all versions instead of
// only the loaded ones
const searchTerm = ref('');
const decision = ref('');
const matches = ref(0);
// incremented with each refresh, so that responses of outdated searches are discarded
let generation = 0;

const trustModelInstance = useWatchedTrustModelInstance(route, refresh, () => historyLimit.value);

const oldestLoadedVersion = computed(() => {
  const versions = Object.keys(trustModelInstance.value?.states || {}).map(Number);
  return versions.length ? Math.min(...versions) : undefined;
});

/*
hasOlder tells whether older versions can be loaded. Without search or filter, versions are numbered from 0 without gaps,
but the oldest loaded ones are dropped while new versions arrive live, so it is derived from the oldest loaded version
instead of from the last fetch. Searches and filters do not drop versions, so their last fetch decides.
*/
const hasOlder = computed(() => {
  if (searchTerm.value || decision.value) {
    return fetchedHasOlder.value;
  }
  const oldest = oldestLoadedVersion.value;
  return oldest !== undefined && oldest > 0 && oldest !== exhaustedBelow.value;
});

// updates never change, so each one is only stringified once
const updateStrings = new WeakMap<object, string>();
function stringifyUpdate(update: any): string {
  if (update === null || typeof update !== 'object') {
    return JSON.stringify(update, null, 2);
  }
  let str = updateStrings.get(update);
  if (str === undefined) {
    str = JSON.stringify(update, null, 2);
    updateStrings.set(update, str);
  }
  return str;
}

const items = computed(() => Object.entries(trustModelInstance.value?.states || {}).map(([k, v]) => ({
  version: Number(k),
  updates: trustModelInstance.value?.updates?.[k]?.map?.(stringifyUpdate),
  atls: trustModelInstance.value?.atls?.[k],
  state: v
})));

const height = ref(document.body.clientHeight - 48);

function onResize() {
  height.value = document.body.clientHeight - 48;
}

// returns the amount of fetched versions
async function fetchVersions(before?: number): Promise<number> {
  const current = generation;
  try {
    const { fetched, matches: total } = await store.fetchTrustModelInstanceVersions(
      route.params.client as string,
      route.params.sessionID as string,
      route.params.template as string,
      route.params.id as string,
      PAGE_SIZE,
      before,
      searchTerm.value,
      decision.value,
      () => current === generation,
    );
    if (current === generation) {
      matches.value = total;
    }
    return fetched;
  } catch {
    router.push('/');
    return 0;
  }
}

async function refresh() {
  generation++;
  // while searching or filtering, the matching versions of the whole history are shown and no new versions are added
  // live, as they may not match
  historyLimit.value = searchTerm.value || decision.value ? undefined : PAGE_SIZE;
  store.clearTrustModelInstanceHistory(`//${route.params.client}/${route.params.sessionID}/${route.params.template}/${route.params.id}`);
  exhaustedBelow.value = undefined;
  fetchedHasOlder.value = await fetchVersions() === PAGE_SIZE;
}

async function loadOlder() {
  const oldest = oldestLoadedVersion.value;
  if (oldest === undefined) {
    return;
  }
  if (historyLimit.value !== undefined) {
    historyLimit.value = Object.keys(trustModelInstance.value?.states || {}).length + PAGE_SIZE;
  }
  const fetched = await fetchVersions(oldest);
  fetchedHasOlder.value = fetched === PAGE_SIZE;
  if (fetched === 0) {
    exhaustedBelow.value = oldest;
  }
}

/*
Loads older versions automatically when the table is scrolled to its end, if it shows the newest versions first (the
default sort order), so that its end are the oldest loaded versions.
*/
let loadingOlder = false;
let scroller: HTMLElement | undefined;

function atEnd(el: HTMLElement): boolean {
  return el.scrollTop + el.clientHeight >= el.scrollHeight - 2 * ROW_HEIGHT;
}

function onScroll(evt: Event) {
  scroller = evt.target as HTMLElement;
  if (atEnd(scroller)) {
    loadOlderAtEnd();
  }
}

async function loadOlderAtEnd() {
  const sortedNewestFirst = sortBy.value[0]?.key === 'version' && sortBy.value[0]?.order === 'desc';
  if (loadingOlder || !hasOlder.value || !sortedNewestFirst) {
    return;
  }
  loadingOlder = true;
  try {
    await loadOlder();
  } finally {
    loadingOlder = false;
  }
  // if the loaded versions do not fill the remaining space, the end is still visible and no scroll event follows
  await nextTick();
  if (scroller && atEnd(scroller)) {
    loadOlderAtEnd();
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined;
function onSearch(term: string) {
  clearTimeout(searchTimer);
  if (term === searchTerm.value) {
    return;
  }
  searchTimer = setTimeout(() => {
    searchTerm.value = term;
    refresh();
  }, 300);
}

function onRemoteFilters(filters: { [key: string]: string }) {
  const value = filters.decision ?? '';
  if (value !== decision.value) {
    decision.value = value;
    refresh();
  }
}
</script>
