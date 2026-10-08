<template>
  <Teleport to="#toolbar">
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
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';

import { TRUST_DECISION_CSS_COLORS, TRUST_DECISION_LABELS, TrustDecision, TrustModelInstance, useAppStore } from '@/stores/app';
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

const items = computed(() => Object.values(store.trustModelInstances));

const height = ref(document.body.clientHeight - 48);

function onResize() {
  height.value = document.body.clientHeight - 48;
}

function openTMI(evt: PointerEvent, {item} : {item: TrustModelInstance}) {
  router.push(`/tmis/${item.fullTMI.replace(/^\/*/, '')}`);
}

</script>
