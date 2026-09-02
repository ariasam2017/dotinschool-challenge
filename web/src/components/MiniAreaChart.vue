<script setup lang="ts">
// A tiny dependency-free area sparkline. Real data, no fake needle gauges.
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{ data: number[]; width?: number; height?: number; color?: string }>(),
  { width: 160, height: 40 },
)

const stroke = computed(() => props.color || 'var(--primary)')

const geom = computed(() => {
  const d = props.data.length ? props.data : [0, 0]
  const max = Math.max(...d, 1)
  const min = Math.min(...d, 0)
  const span = max - min || 1
  const w = props.width
  const h = props.height
  const step = d.length > 1 ? w / (d.length - 1) : w
  const pts = d.map((v, i) => {
    const x = i * step
    const y = h - 3 - ((v - min) / span) * (h - 6)
    return [x, y] as const
  })
  const line = pts.map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`).join(' ')
  const area = `${line} L${w} ${h} L0 ${h} Z`
  return { line, area, last: pts[pts.length - 1] }
})
const gradId = 'mac-' + Math.random().toString(36).slice(2, 8)
</script>

<template>
  <svg
    class="mini-area"
    :viewBox="`0 0 ${width} ${height}`"
    :width="width"
    :height="height"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <defs>
      <linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" :stop-color="stroke" stop-opacity="0.28" />
        <stop offset="100%" :stop-color="stroke" stop-opacity="0" />
      </linearGradient>
    </defs>
    <path :d="geom.area" :fill="`url(#${gradId})`" />
    <path :d="geom.line" fill="none" :stroke="stroke" stroke-width="2" vector-effect="non-scaling-stroke" />
    <circle v-if="geom.last" :cx="geom.last[0]" :cy="geom.last[1]" r="2.4" :fill="stroke" />
  </svg>
</template>

<style scoped>
.mini-area {
  display: block;
  overflow: visible;
}
</style>
