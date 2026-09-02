<script setup lang="ts">
import { computed } from 'vue'
import { diffLabel, diffTone } from '@/lib/fa'
import Chip from './Chip.vue'

const props = defineProps<{ diff: string }>()
const tone = computed(() => diffTone(props.diff))
const label = computed(() => diffLabel(props.diff))
</script>

<template>
  <Chip :tone="tone" dot>{{ label }}</Chip>
</template>
