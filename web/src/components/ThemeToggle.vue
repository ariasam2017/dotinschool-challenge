<script setup lang="ts">
import { computed } from 'vue'
import { themePref, cycleTheme } from '@/lib/theme'
import AppIcon from './AppIcon.vue'

const icon = computed(() =>
  themePref.value === 'light' ? 'sun' : themePref.value === 'dark' ? 'moon' : 'monitor',
)
const label = computed(() =>
  themePref.value === 'light'
    ? 'تم روشن — کلیک: تیره'
    : themePref.value === 'dark'
      ? 'تم تیره — کلیک: پیرو سیستم'
      : 'پیرو سیستم — کلیک: روشن',
)
</script>

<template>
  <button type="button" class="theme-toggle" :title="label" :aria-label="label" @click="cycleTheme()">
    <AppIcon :name="icon" :size="17" />
  </button>
</template>

<style scoped>
.theme-toggle {
  display: inline-grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text-dim);
  transition:
    color var(--dur) var(--ease),
    border-color var(--dur) var(--ease),
    background var(--dur) var(--ease);
}
.theme-toggle:hover {
  color: var(--text);
  border-color: var(--border-strong);
  background: var(--surface-2);
}
</style>
