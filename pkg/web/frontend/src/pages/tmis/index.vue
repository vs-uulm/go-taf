<template>
  <Teleport to="#toolbar">
    <!-- the TMIs as they were at the selected time, colored by the trust decisions of all TMIs over time -->
    <timeline-slider v-model="selectedTime" :min="timeRange.min" :max="timeRange.max" :segments="decisionTimeline" title="Show the TMIs as they were at this time" />

    <faceted-search v-model="filteredItems" :columns="headers" :items="items" sync-to-query v-model:sort-by="sortBy" :defaults="{}" />

    <v-btn icon title="Refresh" size="small" @click="store.fetchTrustModelInstances()" variant="text">
      <v-icon icon="mdi-reload" />
    </v-btn>
  </Teleport>

  <v-data-table-virtual :headers="headers" :items="filteredItems" :height="height" v-resize="onResize" multi-sort v-model:sort-by="sortBy" fixed-header :row-props="stripedRow" @click:row="openTMI">
    <template #[`item.client`]="{ item }">
      <code class="mt-1">{{ item.client }}</code>
    </template>
    <template #[`item.sessionID`]="{ item }">
      <code class="mt-1">{{ item.sessionID }}</code>
    </template>
    <template #[`item.template`]="{ item }">
      <code class="mt-1">{{ item.template }}</code>
    </template>
    <template #[`item.active`]="{ item }">
      <v-checkbox-btn v-model="item.active" readonly />
    </template>
    <template #[`item.latestUpdateTime`]="{ item }">
      <span class="text-no-wrap">{{ formatTafTime(item.latestUpdateTime, store.timeSource) }}</span>
    </template>
    <template #[`item.latestDecisions`]="{ item }">
      <!-- the same chips as for the trust decisions of the ATLs on the TMI page -->
      <template v-if="item.latestDecisions">
        <v-chip v-for="d in decisionsOf(item.latestDecisions)" :key="d" size="x-small" label class="mr-1" :color="TRUST_DECISION_COLORS[d]" variant="flat" :title="item.latestDecisionsVersion === item.latestVersion ? `Trust decision of version ${item.latestVersion}` : `Trust decision of version ${item.latestDecisionsVersion}, as version ${item.latestVersion} has none yet`">{{ TRUST_DECISION_LABELS[d] }}</v-chip>
      </template>
      <span v-else class="text-medium-emphasis">-</span>
    </template>
    <template #[`item.decisionSummary`]="{ item }">
      <!-- a split label: one segment per trust decision, in the colors of the version timeline -->
      <span class="decision-summary">
        <v-tooltip v-for="d in SUMMARY_DECISIONS" :key="d" location="top" :text="decisionSummaryText(item.decisionCounts[d], d)">
          <template #activator="{ props }">
            <span v-bind="props" class="decision-summary__segment" :style="{ backgroundColor: TRUST_DECISION_CSS_COLORS[d] }">{{ item.decisionCounts[d] }}</span>
          </template>
        </v-tooltip>
      </span>
    </template>
  </v-data-table-virtual>
</template>

<style>
/* the shading of Vuetify's striped tables, see stripedRow */
.tmi-row--odd {
  background-image: linear-gradient(0deg, rgba(var(--v-border-color), var(--v-hover-opacity)), rgba(var(--v-border-color), var(--v-hover-opacity)));
}

.decision-summary {
  display: inline-flex;
  border-radius: 4px;
  overflow: hidden;
  font-size: 0.75rem;
  line-height: 1.5rem;
  color: #fff;
}

.decision-summary__segment {
  min-width: 2.25em;
  padding: 0 0.5em;
  text-align: center;
}
</style>

<script lang="ts" setup>
import { computed, onUnmounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import TimelineSlider from '@/components/TimelineSlider.vue';
import { useTimeQuery } from '@/composables/timeQuery';
import { DecisionSegment, formatTafTime, TRUST_DECISION_COLORS, TRUST_DECISION_CSS_COLORS, TRUST_DECISION_LABELS, TRUST_DECISIONS, TrustDecision, TrustModelInstance, useAppStore } from '@/stores/app';
import { Column, SortItem } from '@/types';

const filteredItems = ref<any[]>([]);
const sortBy = ref<SortItem[]>([]);

const router = useRouter();
const store = useAppStore();

// the trust decisions contained in decision bits (1 << TdResults value per decision), in the order of the legend
function decisionsOf(bits: number | undefined): TrustDecision[] {
  return (['TRUSTWORTHY', 'NOT_TRUSTWORTHY', 'UNDECIDABLE'] as TrustDecision[]).filter(d => ((bits ?? 0) & (1 << TRUST_DECISIONS.indexOf(d))) !== 0);
}

// sort rank of decision bits: not trustworthy first, then undecidable, trustworthy, and none
function decisionRank(bits: number | undefined): number {
  const decisions = decisionsOf(bits);
  const rank = (['NOT_TRUSTWORTHY', 'UNDECIDABLE', 'TRUSTWORTHY'] as TrustDecision[]).findIndex(d => decisions.includes(d));
  return rank === -1 ? 3 : rank;
}

const headers: Column[] = [{
  maxWidth: '100px',
  filterable: true,
  title: 'ID',
  key: 'id',
}, {
  maxWidth: '125px',
  filterable: true,
  title: 'Client',
  key: 'client'
}, {
  filterable: true,
  title: 'Session ID',
  key: 'sessionID'
}, {
  filterable: true,
  title: 'Template',
  key: 'template'
}, {
  filterable: true,
  title: 'Active',
  key: 'active'
}, {
  filterable: true,
  title: 'Latest Version',
  key: 'latestVersion'
}, {
  // time of the latest update (or of the spawn), sorted by its value in nanoseconds
  filterable: false,
  title: 'Time of Latest Update',
  key: 'latestUpdateTime'
}, {
  // trust decisions of the shown version; sorted by the most negative decision (not filterable, as the search matches
  // substrings and TRUSTWORTHY is one of NOT_TRUSTWORTHY)
  filterable: false,
  title: 'Latest Decision',
  key: 'latestDecisions',
  sortRaw: (a: TrustModelInstance, b: TrustModelInstance) => decisionRank(a.latestDecisions) - decisionRank(b.latestDecisions)
}, {
  // number of versions whose ATL result set contains each trust decision; sorted by the negative decisions first
  filterable: false,
  title: 'Decision summary',
  key: 'decisionSummary',
  sortRaw: (a: TrustModelInstance, b: TrustModelInstance) =>
    a.decisionCounts.NOT_TRUSTWORTHY - b.decisionCounts.NOT_TRUSTWORTHY
    || a.decisionCounts.UNDECIDABLE - b.decisionCounts.UNDECIDABLE
    || a.decisionCounts.TRUSTWORTHY - b.decisionCounts.TRUSTWORTHY
}];

// segments of the decision summary, in the order of the legend of the version timeline
const SUMMARY_DECISIONS: TrustDecision[] = ['TRUSTWORTHY', 'NOT_TRUSTWORTHY', 'UNDECIDABLE'];

function decisionSummaryText(count: number, decision: TrustDecision): string {
  return `${count} ${count === 1 ? 'version' : 'versions'} led to a${decision === 'UNDECIDABLE' ? 'n' : ''} ${TRUST_DECISION_LABELS[decision]} trust decision`;
}

/*
selectedTime is the time (in nanoseconds, see formatTafTime) at which the TMIs are shown, as fetched from the server
into timedItems; undefined shows the latest state from the store, which is kept up to date live.
*/
const selectedTime = useTimeQuery();
const timedItems = ref<TrustModelInstance[]>([]);

// the timeline spans from the first spawn to the latest update of all TMIs
const timeRange = computed(() => {
  const all = Object.values(store.trustModelInstances);
  const spawns = all.map(tmi => tmi.spawnTime).filter((t): t is number => t !== undefined);
  const updates = all.map(tmi => tmi.latestUpdateTime).filter((t): t is number => t !== undefined);
  if (!spawns.length || !updates.length) {
    return { min: 0, max: 0 };
  }
  return { min: Math.min(...spawns), max: Math.max(...updates) };
});

// incremented with each selection, so that responses for outdated selections are discarded
let generation = 0;
let fetchTimer: ReturnType<typeof setTimeout> | undefined;
watch(selectedTime, (time) => {
  clearTimeout(fetchTimer);
  const current = ++generation;
  if (time === undefined) {
    timedItems.value = [];
    return;
  }
  fetchTimer = setTimeout(async () => {
    const tmis = await store.fetchTrustModelInstancesAt(time);
    if (current === generation) {
      timedItems.value = tmis;
    }
  }, 150);
}, { immediate: true });

/*
The track of the timeline shows the trust decisions of all TMIs over time like the timeline of the visualizer: red while
the current version of any TMI was not trustworthy (which makes the client distrust), green while all decided ones were
trustworthy, grey while there were only undecidable ones. It is fetched again at most once per second while new versions
arrive.
*/
const decisionTimeline = ref<DecisionSegment[]>([]);
const TIMELINE_REFRESH_MS = 1000;
let timelineTimer: ReturnType<typeof setTimeout> | undefined;
let lastTimelineFetch = 0;

async function fetchDecisionTimeline() {
  lastTimelineFetch = Date.now();
  decisionTimeline.value = await store.fetchDecisionTimeline();
}

watch(() => timeRange.value.max, () => {
  if (timelineTimer !== undefined) {
    return;
  }
  timelineTimer = setTimeout(async () => {
    timelineTimer = undefined;
    await fetchDecisionTimeline();
  }, Math.max(0, lastTimelineFetch + TIMELINE_REFRESH_MS - Date.now()));
}, { immediate: true });
onUnmounted(() => clearTimeout(timelineTimer));

const items = computed(() => selectedTime.value === undefined ? Object.values(store.trustModelInstances) : timedItems.value);

const height = ref(document.body.clientHeight - 48);

function onResize() {
  height.value = document.body.clientHeight - 48;
}

/*
stripedRow shades every other row. Vuetify's striped option selects the rendered rows with :nth-child, but the virtual
table only renders the visible rows, so the stripes would swap while scrolling; the index passed here is the position in
the sorted and filtered list instead.
*/
function stripedRow({ index }: { index: number }) {
  return index % 2 === 1 ? { class: 'tmi-row--odd' } : {};
}

// at a selected time, a TMI opens at the same time, i.e., scrolled to its version at that time
function openTMI(evt: PointerEvent, {item} : {item: TrustModelInstance}) {
  const path = `/tmis/${item.fullTMI.replace(/^\/*/, '')}`;
  router.push(selectedTime.value === undefined ? path : { path, query: { at: String(Math.round(selectedTime.value)) } });
}

</script>
