<template>
  <Teleport to="#toolbar">
    <faceted-search v-model="filteredItems" :columns="headers" :items="items" sync-to-query v-model:sort-by="sortBy" :defaults="{}" />

    <v-btn icon title="Refresh" size="small" @click="store.fetchTrustModelInstances()" variant="text">
      <v-icon icon="mdi-reload" />
    </v-btn>
  </Teleport>

  <v-data-table-virtual :headers="headers" :items="filteredItems" :height="height" v-resize="onResize" multi-sort v-model:sort-by="sortBy" @click:row="openTMI">
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
    <template #[`item.decisionCounts.NOT_TRUSTWORTHY`]="{ item }">
      <v-chip v-if="item.decisionCounts.NOT_TRUSTWORTHY > 0" color="error" size="small" variant="flat">{{ item.decisionCounts.NOT_TRUSTWORTHY }}</v-chip>
      <span v-else>0</span>
    </template>
  </v-data-table-virtual>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';

import { TrustModelInstance, useAppStore } from '@/stores/app';
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
  // number of versions whose ATL result set contains the trust decision
  filterable: true,
  title: 'Trustworthy',
  key: 'decisionCounts.TRUSTWORTHY'
}, {
  filterable: true,
  title: 'Not Trustworthy',
  key: 'decisionCounts.NOT_TRUSTWORTHY'
}, {
  filterable: true,
  title: 'Undecidable',
  key: 'decisionCounts.UNDECIDABLE'
}];

const items = computed(() => Object.values(store.trustModelInstances));

const height = ref(document.body.clientHeight - 48);

function onResize() {
  height.value = document.body.clientHeight - 48;
}

function openTMI(evt: PointerEvent, {item} : {item: TrustModelInstance}) {
  router.push(`/tmis/${item.fullTMI.replace(/^\/*/, '')}`);
}

</script>
