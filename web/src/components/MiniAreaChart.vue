<script setup lang="ts">
// A tiny dependency-free area sparkline. Real data, no fake needle gauges.
// Renders at 100% of its container width (viewBox coords are internal), so it
// never ends up clipped or half-drawn inside a flex slot.
import { computed, onMounted, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    data: number[]
    height?: number
    color?: string
    /** draw the stroke on with a short reveal on mount */
    animate?: boolean
  }>(),
  { height: 40, animate: true },
)

// Fixed internal coordinate space; preserveAspectRatio="none" stretches the
// x-axis to the real width. The stroke keeps its pixel weight via
// vector-effect, so horizontal stretch never thickens the line.
const VBW = 100

const stroke = computed(() => props.color || 'var(--primary)')

const geom = computed(() => {
  const d = props.data.length ? props.data : [0, 0]
  const max = Math.max(...d, 1)
  const min = Math.min(...d, 0)
  const span = max - min || 1
  const h = props.height
  const step = d.length > 1 ? VBW / (d.length - 1) : VBW
  const pts = d.map((v, i) => {
    const x = i * step
    const y = h - 3 - ((v - min) / span) * (h - 6)
    return [x, y] as const
  })
  const line = pts.map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(2)} ${y.toFixed(2)}`).join(' ')
  const area = `${line} L${VBW} ${h} L0 ${h} Z`
  return { line, area }
})

const gradId = 'mac-' + Math.random().toString(36).slice(2, 8)

// Reveal: hold the path fully "offset" until mounted, then let the CSS
// transition draw it. pathLength=1 normalises the dash math regardless of the
// real path length.
const revealed = ref(false)
onMounted(() => {
  const reduce =
    typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
  if (!props.animate || reduce) {
    revealed.value = true
    return
  }
  requestAnimationFrame(() => requestAnimationFrame(() => (revealed.value = true)))
})
</script>

<template>
  <svg
    class="mini-area"
    :class="{ revealed }"
    :viewBox="`0 0 ${VBW} ${height}`"
    :height="height"
    width="100%"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <defs>
      <linearGradient :id="gradId" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" :stop-color="stroke" stop-opacity="0.26" />
        <stop offset="100%" :stop-color="stroke" stop-opacity="0" />
      </linearGradient>
    </defs>
    <path class="mac-area" :d="geom.area" :fill="`url(#${gradId})`" />
    <path
      class="mac-line"
      :d="geom.line"
      fill="none"
      :stroke="stroke"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      vector-effect="non-scaling-stroke"
      pathLength="1"
    />
  </svg>
</template>

<style scoped>
.mini-area {
  display: block;
  width: 100%;
  overflow: visible;
}
.mac-line {
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  transition: stroke-dashoffset 0.9s var(--ease);
}
.mac-area {
  opacity: 0;
  transition: opacity 0.5s var(--ease) 0.35s;
}
.mini-area.revealed .mac-line {
  stroke-dashoffset: 0;
}
.mini-area.revealed .mac-area {
  opacity: 1;
}
@media (prefers-reduced-motion: reduce) {
  .mac-line {
    transition: none;
    stroke-dashoffset: 0;
  }
  .mac-area {
    transition: none;
    opacity: 1;
  }
}
</style>
