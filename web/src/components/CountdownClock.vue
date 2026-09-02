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
  gap: 4px;
}
.cd-time {
  font-weight: 700;
  letter-spacing: 0.06em;
  font-variant-numeric: tabular-nums;
  line-height: 1;
  color: var(--text);
  transition: color var(--dur) var(--ease);
}
.cd-md .cd-time {
  font-size: 1.4rem;
}
.cd-lg .cd-time {
  font-size: 2.1rem;
}
.cd-xl .cd-time {
  font-size: clamp(2.4rem, 8vw, 3.4rem);
}
.cd-label {
  font-size: 0.72rem;
  color: var(--text-faint);
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
