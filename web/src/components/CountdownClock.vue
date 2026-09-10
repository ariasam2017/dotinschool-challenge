<script setup lang="ts">
import { computed } from 'vue'
import { fmtRemain } from '@/lib/fa'

type State = 'running' | 'paused' | 'ended' | 'waiting'

const props = withDefaults(
  defineProps<{ ms: number; state?: State; size?: 'md' | 'lg' | 'xl'; label?: string }>(),
  { state: 'running', size: 'lg' },
)

const display = computed(() => fmtRemain(Math.max(0, props.ms)))
const minutesLeft = computed(() => props.ms / 60000)

const tone = computed(() => {
  if (props.state === 'paused') return 'paused'
  if (props.state === 'ended') return 'ended'
  if (props.state === 'waiting') return 'waiting'
  if (minutesLeft.value <= 1) return 'crit'
  if (minutesLeft.value <= 5) return 'warn'
  return 'ok'
})
</script>

<template>
  <div class="cd" :class="[`cd-${size}`, `cd-${tone}`]">
    <div class="cd-time num">{{ display }}</div>
    <div v-if="label" class="cd-label">{{ label }}</div>
  </div>
</template>

<style scoped>
.cd {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}
.cd-time {
  font-weight: 700;
  letter-spacing: 0.04em;
  font-variant-numeric: tabular-nums;
  font-feature-settings: 'tnum' on, 'zero' on;
  line-height: 1;
  color: var(--text);
  transition: color var(--dur) var(--ease);
}
/* final minute: a slow pulse draws the eye without being frantic. Decorative,
   so it collapses to static under reduced-motion. */
@media (prefers-reduced-motion: no-preference) {
  .cd-crit .cd-time {
    animation: cd-pulse 1s var(--ease) infinite;
  }
}
@keyframes cd-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.55; }
}
.cd-md .cd-time {
  font-size: 1.4rem;
}
.cd-lg .cd-time {
  font-size: 2.1rem;
}
.cd-xl .cd-time {
  font-size: clamp(2.6rem, 9vw, 3.8rem);
}
.cd-label {
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--text-faint);
}
.cd-xl .cd-label {
  margin-top: 2px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
  min-width: 12ch;
  text-align: center;
}
.cd-warn .cd-time {
  color: var(--warning);
}
.cd-crit .cd-time {
  color: var(--danger);
}
.cd-paused .cd-time,
.cd-ended .cd-time,
.cd-waiting .cd-time {
  color: var(--text-faint);
}
</style>
