<template>
  <Teleport to="#toolbar">
    <v-btn icon title="Refresh" size="small" @click="refresh" variant="text">
      <v-icon icon="mdi-reload" />
    </v-btn>
  </Teleport>

  <!-- the track is colored by the trust decisions of the versions, its fill is hidden so that the colors stay visible -->
  <v-slider v-model="version" :min="0" :max="trustModelInstance?.latestVersion ?? 0" :step="1" :show-ticks="(trustModelInstance?.latestVersion ?? 0) <= MAX_TICKS ? 'always' : false" tick-size="4" track-size="8" track-fill-color="transparent" class="mt-1 ml-4 decision-slider" :style="{ '--decision-track': decisionTrack }">
    <template #append>
      <div class="d-flex ga-1 mr-2">
        <v-chip v-for="d in LEGEND" :key="d" size="x-small" label variant="flat" :color="TRUST_DECISION_COLORS[d]">{{ TRUST_DECISION_LABELS[d] }}</v-chip>
      </div>
      <v-chip class="pr-0">
        Version
        <v-chip class="ml-2 font-weight-bold">{{ version }}</v-chip>
      </v-chip>
    </template>
  </v-slider>

  <div style="height: calc(100vh - 114px)" v-if="state">
    <trust-graph :state="state" always-show-opinions />
  </div>
</template>

<style>
.decision-slider .v-slider-track__background {
  background: var(--decision-track) !important;
  opacity: 1 !important;
}
</style>

<script lang='ts' setup>
import { computed, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { TRUST_DECISION_COLORS, TRUST_DECISION_CSS_COLORS, TRUST_DECISION_LABELS, TRUST_DECISIONS, TrustDecision, useAppStore, useWatchedTrustModelInstance } from '@/stores/app';

// ticks are only shown for few versions, as each one is a DOM element
const MAX_TICKS = 100;

const LEGEND: TrustDecision[] = ['TRUSTWORTHY', 'NOT_TRUSTWORTHY', 'UNDECIDABLE'];

const route = useRoute();
const store = useAppStore();
const router = useRouter();

const version = computed({
  get() {
    return Number(route.params.version as string);
  },

  set(newVersion) {
    router.replace(`/tmis/${route.params.client}/${route.params.sessionID}/${route.params.template}/${route.params.id}/${newVersion}`);
  }
});

watch(() => route.params.version, () => loadVersion());

const state = computed(() => trustModelInstance.value?.states?.[version.value]);

// color of a version on the track: a negative decision dominates, versions without ATLs stay neutral
function decisionColor(bits: number): string {
  for (const d of ['NOT_TRUSTWORTHY', 'UNDECIDABLE', 'TRUSTWORTHY'] as TrustDecision[]) {
    if (bits & (1 << TRUST_DECISIONS.indexOf(d))) {
      return TRUST_DECISION_CSS_COLORS[d];
    }
  }
  return 'rgba(var(--v-theme-on-surface), 0.12)';
}

/*
decisionTrack is a CSS gradient with a hard-edged segment per run of versions with the same color. Version v sits at
v / latestVersion of the track, so its segment reaches half a version to either side.
*/
const decisionTrack = computed(() => {
  const decisions = trustModelInstance.value?.decisionsByVersion;
  const last = trustModelInstance.value?.latestVersion ?? 0;
  if (!decisions || last === 0) {
    return decisions?.length ? decisionColor(decisions[0]) : 'none';
  }
  const position = (v: number) => `${Math.min(100, Math.max(0, (v - 0.5) / last * 100))}%`;
  const stops: string[] = [];
  let start = 0;
  for (let v = 1; v <= last + 1; v++) {
    if (v === last + 1 || decisionColor(decisions[v] ?? 0) !== decisionColor(decisions[start] ?? 0)) {
      const color = decisionColor(decisions[start] ?? 0);
      stops.push(`${color} ${position(start)}`, `${color} ${position(v)}`);
      start = v;
    }
  }
  return `linear-gradient(to right, ${stops.join(', ')})`;
});

async function refresh() {
  await loadVersion();
  await store.fetchTrustModelInstanceDecisions(
    route.params.client as string,
    route.params.sessionID as string,
    route.params.template as string,
    route.params.id as string
  );
}

async function loadVersion() {
  await store.fetchTrustModelInstance(
    route.params.client as string,
    route.params.sessionID as string,
    route.params.template as string,
    route.params.id as string,
    route.params.version as string
  );
  // only keep the shown version, instead of every version selected with the slider
  store.retainTrustModelInstanceVersions(
    `//${route.params.client}/${route.params.sessionID}/${route.params.template}/${route.params.id}`,
    [Number(route.params.version)]
  );
}

const trustModelInstance = useWatchedTrustModelInstance(route, refresh);
</script>
