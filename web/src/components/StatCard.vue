<script setup lang="ts">
import AppIcon from './AppIcon.vue'

withDefaults(
  defineProps<{
    icon: string
    label: string
    value: string | number
    sub?: string
    tone?: 'primary' | 'info' | 'success' | 'warning' | 'accent'
  }>(),
  { tone: 'primary' },
)
</script>

<template>
  <div class="stat-tile card" :class="`tone-${tone}`">
    <div class="stat-head">
      <span class="stat-ico"><AppIcon :name="icon" :size="18" /></span>
      <span class="stat-label">{{ label }}</span>
    </div>
    <div class="stat-value num">{{ value }}</div>
    <div v-if="sub || $slots.chart" class="stat-foot">
      <span v-if="sub" class="stat-sub">{{ sub }}</span>
      <div v-if="$slots.chart" class="stat-chart"><slot name="chart" /></div>
    </div>
  </div>
</template>

<style scoped>
.stat-tile {
  padding: var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  transition:
    border-color var(--dur) var(--ease),
    box-shadow var(--dur) var(--ease);
}
.stat-tile:hover {
  border-color: var(--border-strong);
  box-shadow: var(--shadow-2);
}
.stat-head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.stat-ico {
  display: inline-grid;
  place-items: center;
  width: 28px;
  height: 28px;
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
.stat-label {
  font-size: 0.8rem;
  color: var(--text-dim);
}
.stat-value {
  font-size: 1.9rem;
  font-weight: 700;
  line-height: 1.1;
  color: var(--text);
}
.stat-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  min-height: 20px;
}
.stat-sub {
  font-size: 0.72rem;
  color: var(--text-faint);
}
.stat-chart {
  flex: 1;
  min-width: 0;
  display: flex;
  justify-content: flex-start;
}
</style>
