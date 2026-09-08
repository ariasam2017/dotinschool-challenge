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
      const langMatch = parts[i].match(/^([a-zA-Z0-9_+-]+)\r?\n/)
      const lang = langMatch ? langMatch[1] : ''
      const block = parts[i].replace(/^[a-zA-Z0-9_+-]*\r?\n/, '').replace(/\r?\n$/, '')
      out +=
        `<div class="md-codewrap">` +
        `<div class="md-codebar"><span class="md-lang">${esc(lang || 'code')}</span>` +
        `<button class="md-copy" type="button">کپی همه</button></div>` +
        `<pre class="md-code">${esc(block)}</pre></div>`
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

async function copyCode(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    return
  } catch {
    /* clipboard API blocked (insecure context / permissions) — fall back */
  }
  const ta = document.createElement('textarea')
  ta.value = text
  ta.setAttribute('readonly', '')
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
  } catch {
    /* ignore — nothing more we can do */
  }
  ta.remove()
}

// The markdown is injected via v-html, so the copy buttons can't carry Vue
// listeners — delegate from the container instead.
async function onClick(e: MouseEvent): Promise<void> {
  const btn = (e.target as HTMLElement).closest('.md-copy') as HTMLButtonElement | null
  if (!btn) return
  const pre = btn.closest('.md-codewrap')?.querySelector('pre.md-code')
  if (!pre) return
  await copyCode(pre.textContent ?? '')
  const prev = btn.dataset.label || btn.textContent || 'کپی همه'
  btn.dataset.label = prev
  btn.textContent = 'کپی شد ✓'
  btn.classList.add('done')
  window.setTimeout(() => {
    btn.textContent = prev
    btn.classList.remove('done')
  }, 1600)
}
</script>

<template>
  <div class="md" @click="onClick" v-html="html" />
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
.md :deep(.md-codewrap) {
  margin: 0 0 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}
.md :deep(.md-codebar) {
  direction: ltr;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 6px 4px 12px;
  background: var(--surface-2);
  border-bottom: 1px solid var(--border);
}
.md :deep(.md-lang) {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.03em;
  color: var(--text-faint);
  text-transform: lowercase;
}
.md :deep(.md-copy) {
  font-family: var(--font-ui);
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-dim);
  background: var(--surface);
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  padding: 3px 12px;
  cursor: pointer;
  white-space: nowrap;
  transition:
    color var(--dur) var(--ease),
    border-color var(--dur) var(--ease),
    background var(--dur) var(--ease);
}
.md :deep(.md-copy:hover) {
  color: var(--text);
  border-color: var(--primary-bright);
}
.md :deep(.md-copy:active) {
  background: var(--surface-2);
}
.md :deep(.md-copy.done) {
  color: var(--on-primary);
  background: var(--primary);
  border-color: var(--primary);
}
.md :deep(.md-code) {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  line-height: 1.7;
  background: var(--code-bg);
  color: var(--code-text);
  padding: 12px 14px;
  overflow-x: auto;
  margin: 0;
  direction: ltr;
  text-align: left;
}
.md :deep(strong) {
  color: var(--text);
  font-weight: 700;
}
</style>
