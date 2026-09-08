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
    <div class="stat-top">
      <span class="stat-label">{{ label }}</span>
      <span class="stat-ico"><AppIcon :name="icon" :size="16" /></span>
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
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--space-3);
  min-height: 22px;
  margin-top: auto;
}
.stat-sub {
  font-size: 0.72rem;
  color: var(--text-faint);
}
.stat-chart {
  flex: 1;
  min-width: 0;
  display: flex;
  justify-content: flex-end;
  opacity: 0.9;
}
</style>
