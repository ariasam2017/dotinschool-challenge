<script setup lang="ts">
withDefaults(
  defineProps<{
    tone?: 'neutral' | 'info' | 'success' | 'warning' | 'danger' | 'accent'
    dot?: boolean
  }>(),
  { tone: 'neutral', dot: false },
)
</script>

<template>
  <span class="chip" :class="`chip-${tone}`">
    <span v-if="dot" class="chip-dot" aria-hidden="true" />
    <slot />
  </span>
</template>

<style scoped>
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  font-size: 0.75rem;
  font-weight: 600;
  line-height: 1.6;
  white-space: nowrap;
  border: 1px solid transparent;
}
.chip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.chip-neutral {
  background: var(--surface-2);
  color: var(--text-dim);
  border-color: var(--border);
}
.chip-info {
  background: var(--info-soft);
  color: var(--info);
}
.chip-success {
  background: var(--success-soft);
  color: var(--success);
}
.chip-warning {
  background: var(--warning-soft);
  color: var(--warning);
}
.chip-danger {
  background: var(--danger-soft);
  color: var(--danger);
}
.chip-accent {
  background: var(--accent-soft);
  color: var(--accent);
}
</style>
