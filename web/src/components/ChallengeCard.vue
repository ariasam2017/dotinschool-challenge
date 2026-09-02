<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Challenge } from '@/api/types'
import { toFa } from '@/lib/fa'
import DiffChip from './DiffChip.vue'
import Chip from './Chip.vue'
import AppIcon from './AppIcon.vue'

const props = withDefaults(defineProps<{ challenge: Challenge; compact?: boolean }>(), {
  compact: false,
})

const paragraphs = computed(() => (props.challenge.desc || '').split('\n\n').filter(Boolean))
const modeText = computed(() =>
  props.challenge.mode === 'performance'
    ? `امتیازدهی بر اساس سرعت اجرا — هدف: ${toFa(props.challenge.timeLimitMs || 200)} میلی‌ثانیه`
    : 'امتیازدهی بر اساس درستی خروجی',
)

const lightbox = ref(false)
</script>

<template>
  <article class="challenge" :class="{ compact }">
    <img
      v-if="challenge.image"
      :src="challenge.image"
      alt="تصویر پیوست چالش"
      class="challenge-img"
      @click="lightbox = true"
    />

    <header class="challenge-head">
      <div class="row-wrap">
        <DiffChip :diff="challenge.diff" />
        <Chip tone="neutral">{{ challenge.tag }}</Chip>
      </div>
      <h2 class="challenge-title">{{ challenge.title }}</h2>
    </header>

    <div class="challenge-body">
      <p v-for="(p, i) in paragraphs" :key="i">{{ p }}</p>
    </div>

    <div class="io-grid">
      <div class="io-block">
        <div class="io-label">ورودی نمونه</div>
        <code class="io-code num">{{ challenge.sampleIn }}</code>
      </div>
      <div class="io-block">
        <div class="io-label">خروجی نمونه</div>
        <code class="io-code num">{{ challenge.sampleOut }}</code>
      </div>
    </div>

    <div class="mode-badge">
      <span class="mode-ico"><AppIcon :name="challenge.mode === 'performance' ? 'zap' : 'check-circle'" :size="15" /></span>
      {{ modeText }}
    </div>

    <Teleport to="body">
      <div v-if="lightbox && challenge.image" class="lightbox" @click="lightbox = false">
        <img :src="challenge.image" alt="تصویر چالش" />
        <button type="button" class="lightbox-close" aria-label="بستن"><AppIcon name="x" :size="20" /></button>
      </div>
    </Teleport>
  </article>
</template>

<style scoped>
.challenge {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.challenge-img {
  width: 100%;
  max-height: 260px;
  object-fit: cover;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  cursor: zoom-in;
}
.challenge-title {
  font-size: 1.35rem;
  margin: 8px 0 0;
}
.compact .challenge-title {
  font-size: 1.05rem;
}
.challenge-body p {
  font-size: 0.92rem;
  line-height: 2;
  color: var(--text-dim);
  margin: 0 0 8px;
}
.compact .challenge-body p {
  font-size: 0.84rem;
  line-height: 1.9;
}
.io-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-3);
}
@media (min-width: 640px) {
  .io-grid {
    grid-template-columns: 1fr 1fr;
  }
}
.io-block {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
}
.io-label {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-faint);
  margin-bottom: 6px;
}
.io-code {
  display: block;
  font-size: 0.82rem;
  color: var(--text);
  direction: ltr;
  text-align: left;
  white-space: pre-wrap;
  word-break: break-word;
}
.mode-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  align-self: flex-start;
  padding: 6px 12px;
  border-radius: var(--radius-pill);
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 0.78rem;
  font-weight: 600;
}
.mode-ico {
  display: inline-grid;
  place-items: center;
}
.lightbox {
  position: fixed;
  inset: 0;
  z-index: 2000;
  display: grid;
  place-items: center;
  background: rgba(2, 6, 23, 0.82);
  backdrop-filter: blur(4px);
  padding: var(--space-5);
  cursor: zoom-out;
}
.lightbox img {
  max-width: min(92vw, 1000px);
  max-height: 88vh;
  border-radius: var(--radius);
}
.lightbox-close {
  position: absolute;
  top: 20px;
  inset-inline-end: 20px;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  border: none;
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
  display: grid;
  place-items: center;
}
</style>
