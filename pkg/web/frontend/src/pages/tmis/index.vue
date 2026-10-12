<template>
  <Teleport to="#toolbar">
    <!-- the timeline shows the TMIs as they were at the selected time, or live at its end -->
    <div class="d-flex align-center flex-grow-1 mx-2" style="min-width: 200px">
      <v-slider v-model="sliderTime" :min="timeRange.min" :max="timeRange.max" :step="TIME_STEP" :disabled="timeRange.max <= timeRange.min" hide-details density="compact" color="primary" class="flex-grow-1" title="Show the TMIs as they were at this time" />
      <v-chip label size="small" class="ml-3 text-no-wrap" prepend-icon="mdi-clock-outline" :color="selectedTime === undefined ? 'success' : undefined" :title="selectedTime === undefined ? 'The latest state of the TMIs is shown' : 'The TMIs are shown as they were at this time'">
        {{ selectedTime === undefined ? 'Live' : formatTafTime(selectedTime, store.timeSource) }}
      </v-chip>
      <v-btn icon size="small" variant="text" :disabled="selectedTime === undefined" title="Show the latest state of the TMIs (live)" @click="selectedTime = undefined">
        <v-icon icon="mdi-skip-forward" />
      </v-btn>
    </div>

    <faceted-search v-model="filteredItems" :columns="headers" :items="items" sync-to-query v-model:sort-by="sortBy" :defaults="{}" />

    <v-btn icon title="Refresh" size="small" @click="store.fetchTrustModelInstances()" variant="text">
      <v-icon icon="mdi-reload" />
    </v-btn>
  </Teleport>

  <v-data-table-virtual :headers="headers" :items="filteredItems" :height="height" v-resize="onResize" multi-sort v-model:sort-by="sortBy" fixed-header @click:row="openTMI">
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
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { formatTafTime, TRUST_DECISION_CSS_COLORS, TRUST_DECISION_LABELS, TrustDecision, TrustModelInstance, useAppStore } from '@/stores/app';
import { Column, SortItem } from '@/types';

const filteredItems = ref<any[]>([]);
const sortBy = ref<SortItem[]>([]);

const router = useRouter();
const store = useAppStore();
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

// times are selected in steps of 1 ms (in nanoseconds)
const TIME_STEP = 1e6;

/*
selectedTime is the time (in nanoseconds, see formatTafTime) at which the TMIs are shown, as fetched from the server
into timedItems; undefined shows the latest state from the store, which is kept up to date live.
*/
const selectedTime = ref<number | undefined>(undefined);
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

// moving the slider to its end shows the latest state again
const sliderTime = computed({
  get: () => selectedTime.value ?? timeRange.value.max,
  set: (time: number) => {
    selectedTime.value = time >= timeRange.value.max ? undefined : time;
  }
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
});

const items = computed(() => selectedTime.value === undefined ? Object.values(store.trustModelInstances) : timedItems.value);

const height = ref(document.body.clientHeight - 48);

function onResize() {
  height.value = document.body.clientHeight - 48;
}

// at a selected time, a TMI opens with its version at that time
function openTMI(evt: PointerEvent, {item} : {item: TrustModelInstance}) {
  const path = `/tmis/${item.fullTMI.replace(/^\/*/, '')}`;
  router.push(selectedTime.value === undefined ? path : `${path}/${item.latestVersion}`);
}

</script>
