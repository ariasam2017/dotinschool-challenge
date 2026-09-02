<script setup lang="ts">
import { computed } from 'vue'
import faIR from 'ant-design-vue/es/locale/fa_IR'
import { antdTheme } from '@/lib/theme'
import { APP_MODE } from '@/lib/mode'

const locale = faIR
const panelClass = computed(() => `panel-${APP_MODE}`)
</script>

<template>
  <a-config-provider :theme="antdTheme" :locale="locale" direction="rtl">
    <a-app :class="panelClass">
      <router-view />
    </a-app>
  </a-config-provider>
</template>

<style>
.ant-app {
  min-height: 100dvh;
  color: var(--text);
}
</style>
