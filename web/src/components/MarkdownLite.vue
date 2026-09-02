<script setup lang="ts">
// Minimal, XSS-safe Markdown: everything is escaped first, then only a fixed set
// of patterns (headings, lists, bold, inline code, fenced code) becomes HTML.
// No raw HTML from the AI response is ever executed. Ported from admin.js.
import { computed } from 'vue'

const props = defineProps<{ source: string }>()

function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
function inline(t: string): string {
  return esc(t)
    .replace(/`([^`]+)`/g, '<code class="md-ic">$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
}

const html = computed(() => {
  const parts = String(props.source ?? '').split('```')
  let out = ''
  for (let i = 0; i < parts.length; i++) {
    if (i % 2 === 1) {
      const block = parts[i].replace(/^[a-zA-Z0-9_+-]*\r?\n/, '').replace(/\r?\n$/, '')
      out += `<pre class="md-code">${esc(block)}</pre>`
      continue
    }
    let list: 'ul' | 'ol' | null = null
    const closeList = () => {
      if (list) {
        out += `</${list}>`
        list = null
      }
    }
    for (const raw of parts[i].split('\n')) {
      const line = raw.replace(/\r$/, '')
      let m: RegExpMatchArray | null
      if (/^\s*#{1,6}\s+/.test(line)) {
        closeList()
        out += `<h4 class="md-h">${inline(line.replace(/^\s*#{1,6}\s+/, ''))}</h4>`
      } else if ((m = line.match(/^\s*[-*]\s+(.*)$/))) {
        if (list !== 'ul') {
          closeList()
          out += '<ul class="md-list">'
          list = 'ul'
        }
        out += `<li>${inline(m[1])}</li>`
      } else if ((m = line.match(/^\s*\d+[.)]\s+(.*)$/))) {
        if (list !== 'ol') {
          closeList()
          out += '<ol class="md-list">'
          list = 'ol'
        }
        out += `<li>${inline(m[1])}</li>`
      } else if (line.trim() === '') {
        closeList()
      } else {
        closeList()
        out += `<p>${inline(line)}</p>`
      }
    }
    closeList()
  }
  return out
})
</script>

<template>
  <div class="md" v-html="html" />
</template>

<style scoped>
.md :deep(.md-h) {
  font-size: 0.92rem;
  font-weight: 700;
  margin: 16px 0 6px;
  color: var(--text);
}
.md :deep(p) {
  margin: 0 0 8px;
  font-size: 0.86rem;
  line-height: 1.9;
  color: var(--text-dim);
}
.md :deep(.md-list) {
  margin: 0 0 10px;
  padding-inline-start: 20px;
}
.md :deep(.md-list li) {
  font-size: 0.86rem;
  line-height: 1.85;
  color: var(--text-dim);
}
.md :deep(.md-ic) {
  font-family: var(--font-mono);
  font-size: 0.82em;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 1px 5px;
}
.md :deep(.md-code) {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  line-height: 1.7;
  background: var(--code-bg);
  color: var(--code-text);
  border-radius: var(--radius-sm);
  padding: 12px 14px;
  overflow-x: auto;
  margin: 0 0 12px;
  direction: ltr;
  text-align: left;
}
.md :deep(strong) {
  color: var(--text);
  font-weight: 700;
}
</style>
