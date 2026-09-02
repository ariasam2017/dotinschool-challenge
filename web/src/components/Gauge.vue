<script setup lang="ts">
import { computed } from 'vue'
import { toFa } from '@/lib/fa'

const props = withDefaults(
  defineProps<{
    percent: number
    label?: string
    color?: string
    size?: number
    showValue?: boolean
    valueSuffix?: string
  }>(),
  { size: 96, showValue: true, valueSuffix: '٪' },
)

const R = 34
const C = 2 * Math.PI * R
const clamped = computed(() => Math.max(0, Math.min(100, props.percent || 0)))
const offset = computed(() => C - (clamped.value / 100) * C)
const stroke = computed(() => props.color || 'var(--primary)')
</script>

<template>
  <div class="gauge" :style="{ width: size + 'px' }">
    <svg :width="size" :height="size" viewBox="0 0 80 80" class="gauge-svg" aria-hidden="true">
      <circle cx="40" cy="40" :r="R" fill="none" stroke="var(--surface-3)" stroke-width="7" />
      <circle
        cx="40"
        cy="40"
        :r="R"
        fill="none"
        :stroke="stroke"
        stroke-width="7"
        stroke-linecap="round"
        :stroke-dasharray="C"
        :stroke-dashoffset="offset"
        transform="rotate(-90 40 40)"
        class="gauge-arc"
      />
    </svg>
    <div v-if="showValue" class="gauge-center num">{{ toFa(Math.round(clamped)) }}{{ valueSuffix }}</div>
    <div v-if="label" class="gauge-label">{{ label }}</div>
  </div>
</template>

<style scoped>
.gauge {
  position: relative;
  display: inline-grid;
  place-items: center;
  text-align: center;
}
.gauge-svg {
  display: block;
}
.gauge-arc {
  transition: stroke-dashoffset 0.6s var(--ease);
}
.gauge-center {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-weight: 700;
  font-size: 0.95rem;
  color: var(--text);
}
.gauge-label {
  margin-top: 6px;
  font-size: 0.72rem;
  color: var(--text-faint);
}
</style>
