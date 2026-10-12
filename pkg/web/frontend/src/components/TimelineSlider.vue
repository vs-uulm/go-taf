<template>
  <!-- shows the selected time, or live at the end of the timeline -->
  <div class="d-flex align-center flex-grow-1 mx-2" style="min-width: 200px">
    <!-- the track is colored by the trust decisions over time, its fill is hidden so that they stay visible -->
    <v-slider v-model="sliderTime" :min="min" :max="max" :step="TIME_STEP" :disabled="max <= min" hide-details density="compact" color="primary" track-size="6" track-fill-color="transparent" class="flex-grow-1 timeline-slider" :style="{ '--timeline-track': track }" :title="`${title} (red: not trustworthy, green: trustworthy, grey: undecidable only)`" />
    <v-chip label size="small" class="ml-3 text-no-wrap" prepend-icon="mdi-clock-outline" :color="modelValue === undefined ? 'success' : undefined" :title="modelValue === undefined ? 'The latest state is shown' : 'The state at this time is shown'">
      {{ modelValue === undefined ? 'Live' : formatTafTime(modelValue, store.timeSource) }}
    </v-chip>
    <v-btn icon size="small" variant="text" :disabled="modelValue === undefined" title="Show the latest state (live)" @click="emit('update:modelValue', undefined)">
      <v-icon icon="mdi-skip-forward" />
    </v-btn>
  </div>
</template>

<style>
.timeline-slider .v-slider-track__background {
  background: var(--timeline-track) !important;
  opacity: 1 !important;
}
</style>

<script lang="ts" setup>
import { computed } from 'vue';

import { DecisionSegment, formatTafTime, TRUST_DECISION_CSS_COLORS, TRUST_DECISIONS, TrustDecision, useAppStore } from '@/stores/app';

/*
TimelineSlider selects a time (in nanoseconds, see formatTafTime) between min and max, or undefined for the latest
(live) state, which is also selected by moving it to its end. Its track is colored by the trust decisions over time,
given as segments: red while not trustworthy, green while trustworthy, grey while undecidable only.
*/
const props = defineProps<{
  modelValue: number | undefined,
  min: number,
  max: number,
  segments: DecisionSegment[],
  title: string,
}>();

const emit = defineEmits<{
  'update:modelValue': [time: number | undefined]
}>();

const store = useAppStore();

// times are selected in steps of 1 ms (in nanoseconds)
const TIME_STEP = 1e6;

const sliderTime = computed({
  get: () => props.modelValue ?? props.max,
  set: (time: number) => emit('update:modelValue', time >= props.max ? undefined : time)
});

const NEUTRAL_TRACK_COLOR = 'rgba(var(--v-theme-on-surface), 0.12)';

function color(bits: number): string {
  for (const d of ['NOT_TRUSTWORTHY', 'TRUSTWORTHY', 'UNDECIDABLE'] as TrustDecision[]) {
    if (bits & (1 << TRUST_DECISIONS.indexOf(d))) {
      return TRUST_DECISION_CSS_COLORS[d];
    }
  }
  return NEUTRAL_TRACK_COLOR;
}

// a CSS gradient with a hard-edged segment per change of the color
const track = computed(() => {
  const { min, max, segments } = props;
  if (max <= min || !segments.length) {
    return NEUTRAL_TRACK_COLOR;
  }
  const position = (t: number) => `${Math.min(100, Math.max(0, (t - min) / (max - min) * 100))}%`;
  const stops: string[] = [`${NEUTRAL_TRACK_COLOR} 0%`, `${NEUTRAL_TRACK_COLOR} ${position(segments[0].time)}`];
  segments.forEach((segment, i) => {
    const c = color(segment.decisions);
    stops.push(`${c} ${position(segment.time)}`, `${c} ${position(segments[i + 1]?.time ?? max)}`);
  });
  return `linear-gradient(to right, ${stops.join(', ')})`;
});
</script>
