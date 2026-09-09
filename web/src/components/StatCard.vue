<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import AppIcon from './AppIcon.vue'
import { toFa } from '@/lib/fa'

const props = withDefaults(
  defineProps<{
    icon: string
    label: string
    /** a number is counted up on mount and on every change; a string is shown as-is */
    value: string | number
    sub?: string
    tone?: 'primary' | 'info' | 'success' | 'warning' | 'accent'
    /** optional live trend badge shown in the footer */
    trend?: 'up' | 'down' | 'flat'
    trendText?: string
  }>(),
  { tone: 'primary' },
)

/* ---- count-up ---- */
const display = ref('')
let raf = 0

function render(n: number) {
  display.value = toFa(Math.round(n))
}
function animateTo(target: number, from: number) {
  const reduce =
    typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
  if (reduce || from === target) {
    render(target)
    return
  }
  cancelAnimationFrame(raf)
  const dur = 620
  const t0 = performance.now()
  const tick = (now: number) => {
    const p = Math.min(1, (now - t0) / dur)
    const eased = 1 - Math.pow(1 - p, 3) // easeOutCubic
    render(from + (target - from) * eased)
    if (p < 1) raf = requestAnimationFrame(tick)
  }
  raf = requestAnimationFrame(tick)
}

/* a soft tint flash when the value changes under polling */
const bump = ref(false)
let bumpT = 0

onMounted(() => {
  if (typeof props.value === 'number') animateTo(props.value, 0)
  else display.value = String(props.value)
})
watch(
  () => props.value,
  (v, old) => {
    if (typeof v === 'number') animateTo(v, typeof old === 'number' ? old : 0)
    else display.value = String(v)
    bump.value = true
    clearTimeout(bumpT)
    bumpT = window.setTimeout(() => (bump.value = false), 600)
  },
)
</script>

<template>
  <div class="stat-tile card" :class="[`tone-${tone}`, { bump }]">
    <div class="stat-top">
      <span class="stat-label">{{ label }}</span>
      <span class="stat-ico"><AppIcon :name="icon" :size="16" /></span>
    </div>
    <div class="stat-value num">{{ display }}</div>
    <div v-if="sub || trend || $slots.chart" class="stat-foot">
      <div v-if="sub || trend" class="stat-foot-row">
        <span v-if="trend" class="stat-trend" :class="`t-${trend}`">
          <AppIcon
            :name="trend === 'up' ? 'trending-up' : trend === 'down' ? 'trending-down' : 'minus'"
            :size="13"
          />
          <span v-if="trendText">{{ trendText }}</span>
        </span>
        <span v-else-if="sub" class="stat-sub">{{ sub }}</span>
      </div>
      <div v-if="$slots.chart" class="stat-chart"><slot name="chart" /></div>
    </div>
  </div>
</template>

<style scoped>
.stat-tile {
  position: relative;
  padding: var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  overflow: hidden;
  transition:
    border-color var(--dur) var(--ease),
    box-shadow var(--dur) var(--ease),
    transform var(--dur) var(--ease);
}
/* tone accent bar along the top edge */
.stat-tile::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  background: var(--primary);
}
.tone-info.stat-tile::before {
  background: var(--info);
}
.tone-success.stat-tile::before {
  background: var(--success);
}
.tone-warning.stat-tile::before {
  background: var(--warning);
}
.tone-accent.stat-tile::before {
  background: var(--accent);
}
.stat-tile:hover {
  border-color: var(--border-strong);
  box-shadow: var(--shadow-2);
  transform: translateY(-1px);
}
/* brief tint pulse when the number updates under polling */
.stat-tile.bump {
  animation: stat-bump 0.6s var(--ease);
}
@keyframes stat-bump {
  0% {
    background: var(--surface);
  }
  30% {
    background: var(--primary-soft);
  }
  100% {
    background: var(--surface);
  }
}
.tone-info.bump {
  animation-name: stat-bump-info;
}
.tone-success.bump {
  animation-name: stat-bump-success;
}
.tone-accent.bump {
  animation-name: stat-bump-accent;
}
@keyframes stat-bump-info {
  30% {
    background: var(--info-soft);
  }
  0%,
  100% {
    background: var(--surface);
  }
}
@keyframes stat-bump-success {
  30% {
    background: var(--success-soft);
  }
  0%,
  100% {
    background: var(--surface);
  }
}
@keyframes stat-bump-accent {
  30% {
    background: var(--accent-soft);
  }
  0%,
  100% {
    background: var(--surface);
  }
}
@media (prefers-reduced-motion: reduce) {
  .stat-tile.bump {
    animation: none;
  }
}
.stat-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
.stat-label {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-dim);
}
.stat-ico {
  display: inline-grid;
  place-items: center;
  width: 26px;
  height: 26px;
  flex: none;
  border-radius: var(--radius-sm);
  background: var(--primary-soft);
  color: var(--primary-bright);
}
.tone-info .stat-ico {
  background: var(--info-soft);
  color: var(--info);
}
.tone-success .stat-ico {
  background: var(--success-soft);
  color: var(--success);
}
.tone-warning .stat-ico {
  background: var(--warning-soft);
  color: var(--warning);
}
.tone-accent .stat-ico {
  background: var(--accent-soft);
  color: var(--accent);
}
.stat-value {
  font-size: 2rem;
  font-weight: 800;
  line-height: 1.05;
  letter-spacing: -0.01em;
  color: var(--text);
}
.stat-foot {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: var(--space-2);
  min-height: 22px;
  margin-top: auto;
}
.stat-foot-row {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-3);
}
.stat-sub {
  font-size: 0.72rem;
  line-height: 1.5;
  color: var(--text-faint);
}
.stat-trend {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.72rem;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}
.stat-trend.t-up {
  color: var(--success);
  background: var(--success-soft);
}
.stat-trend.t-down {
  color: var(--danger);
  background: var(--danger-soft);
}
.stat-trend.t-flat {
  color: var(--text-faint);
  background: var(--surface-2);
}
.stat-chart {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  min-width: 0;
}
</style>
