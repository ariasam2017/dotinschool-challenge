<script setup lang="ts">
import { computed } from 'vue'
import { statusLabel, statusTone } from '@/lib/fa'
import Chip from './Chip.vue'

const props = defineProps<{ status: string }>()
const tone = computed(() => statusTone(props.status))
const label = computed(() => statusLabel(props.status))
</script>

<template>
  <Chip :tone="tone" dot>{{ label }}</Chip>
</template>
