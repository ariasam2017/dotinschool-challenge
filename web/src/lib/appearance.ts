// Design Studio runtime — turns the admin-authored Appearance object into a
// live <style> tag on the user panel, and (in an admin preview iframe) accepts
// draft updates over postMessage so every slider drag shows instantly.
//
// Nothing here trusts its input: values are re-sanitised client-side before
// they reach the DOM, exactly like the server does, because preview payloads
// arrive straight from the admin form without a server round-trip.

import { reactive } from 'vue'
import type { Appearance, TextReplacement, ElementOverride } from '@/api/types'

export const isPreview =
  typeof location !== 'undefined' && new URLSearchParams(location.search).has('preview')

// Text overrides components read reactively (Vue escapes these on render).
export const appearanceText = reactive<{
  heroTitle: string
  heroSubtitle: string
  heroShowSubtitle: boolean
  brand: string
}>({
  heroTitle: '',
  heroSubtitle: '',
  heroShowSubtitle: true,
  brand: '',
})

const STYLE_ID = 'dotin-appearance'
const TOKEN_KEYS = new Set([
  'primary', 'primary-hover', 'primary-soft', 'accent', 'accent-soft',
  'bg', 'surface', 'surface-2', 'surface-3', 'text', 'text-dim', 'text-faint',
  'border', 'border-strong', 'danger', 'warning', 'success', 'info',
  'on-primary', 'on-accent', 'ring', 'radius', 'radius-sm', 'radius-lg',
  'radius-pill', 'font-ui', 'shadow-1', 'shadow-2', 'shadow-3',
  'space-3', 'space-4', 'space-5', 'space-6', 'header-h', 'sidebar-w',
  'code-bg', 'code-text',
])

const VALUE_RE = /^[A-Za-z0-9#%.,()\-_'"/ ]+$/
const SELECTOR_RE = /^[A-Za-z0-9 .#:>()[\]="'\-_,\s]+$/
const BAD_URL_RE = /url\(\s*['"]?\s*(?:https?:|\/\/|\/)/i
const CSS_BLOCK_RE =
  /(@import|<\s*\/\s*style|<\s*script|javascript:|expression\s*\(|-moz-binding|url\(\s*['"]?\s*(?:https?:|\/\/))/i
const IMAGE_RE = /^data:image\/(png|jpe?g|webp|gif|svg\+xml);base64,[A-Za-z0-9+/=\s]+$/

function safeValue(v: unknown): string {
  const s = String(v ?? '').trim()
  if (!s || s.length > 200 || !VALUE_RE.test(s) || BAD_URL_RE.test(s)) return ''
  return s
}

function tokenBlock(selector: string, map?: Record<string, string>): string {
  if (!map) return ''
  const decls: string[] = []
  for (const [rawKey, rawVal] of Object.entries(map)) {
    const key = String(rawKey).replace(/^--/, '').trim()
    if (!TOKEN_KEYS.has(key)) continue
    const val = safeValue(rawVal)
    if (val) decls.push(`--${key}:${val} !important`)
  }
  return decls.length ? `${selector}{${decls.join(';')}}` : ''
}

/** Assemble the full stylesheet text for an Appearance. */
export function buildCss(a: Appearance | null | undefined): string {
  if (!a) return ''
  const out: string[] = []

  out.push(tokenBlock(':root', a.tokens))
  const dark = tokenBlock(':root[data-theme="dark"]', a.dark)
  if (dark) {
    out.push(dark)
    out.push(
      `@media (prefers-color-scheme:dark){${tokenBlock(
        ':root:not([data-theme="light"])',
        a.dark,
      )}}`,
    )
  }

  const hero = a.sections?.hero
  if (hero) {
    const h: string[] = []
    const bg = safeValue(hero.background)
    if (bg) h.push(`background:${bg}`)
    const img = String(hero.image ?? '').trim()
    if (img && img.length <= 512 * 1024 && IMAGE_RE.test(img)) {
      h.push(`background-image:url("${img}")`, 'background-size:cover', 'background-position:center')
    }
    const mh = safeValue(hero.minHeight)
    if (mh) h.push(`min-height:${mh}`)
    if (h.length) out.push(`.hero{${h.join(';')}}`)
    if (hero.align === 'start' || hero.align === 'center' || hero.align === 'end') {
      out.push(`.hero-copy,.hero-copy h1,.hero-sub{text-align:${hero.align}}`)
    }
  }

  const header = a.sections?.header
  const hbg = safeValue(header?.background)
  if (hbg) out.push(`.u-header{background:${hbg}}`)

  for (const o of a.overrides || []) {
    if (!o.selector || o.selector.length > 400 || !SELECTOR_RE.test(o.selector)) continue
    const d: string[] = []
    if (o.hidden) {
      d.push('display:none')
    } else {
      if (o.fontSize && /^[0-9]{1,3}(\.[0-9]{1,2})?(px|rem|em|%)$/.test(o.fontSize)) {
        d.push(`font-size:${o.fontSize}`)
      }
      if (o.fontWeight && /^(300|400|500|600|700|800|900|normal|bold)$/.test(String(o.fontWeight))) {
        d.push(`font-weight:${o.fontWeight}`)
      }
      if (o.textAlign && /^(right|left|center|justify)$/.test(o.textAlign)) {
        d.push(`text-align:${o.textAlign}`)
      }
      const css = safeValue(o.color)
      if (css) d.push(`color:${css}`)
      const bg = safeValue(o.background)
      if (bg) d.push(`background:${bg}`)
      const pad = safeValue(o.padding)
      if (pad) d.push(`padding:${pad}`)
      const mar = safeValue(o.margin)
      if (mar) d.push(`margin:${mar}`)
      const bw = safeValue(o.borderWidth)
      if (bw) d.push(`border-width:${bw}`, 'border-style:solid')
      const bc = safeValue(o.borderColor)
      if (bc) d.push(`border-color:${bc}`)
      const br = safeValue(o.borderRadius)
      if (br) d.push(`border-radius:${br}`)
      const w = safeValue(o.width)
      if (w) d.push(`width:${w}`)
      const mw = safeValue(o.maxWidth)
      if (mw) d.push(`max-width:${mw}`)
    }
    if (d.length) out.push(`${o.selector}{${d.map((x) => x + ' !important').join(';')}}`)
  }

  const custom = String(a.customCss ?? '').trim()
  if (custom && custom.length <= 20000 && !CSS_BLOCK_RE.test(custom)) {
    out.push(`/* custom */\n${custom}`)
  }

  return out.filter(Boolean).join('\n')
}

/** Apply an Appearance: swap the <style> tag and update the text overrides. */
export function applyAppearance(a: Appearance | null | undefined): void {
  if (typeof document === 'undefined') return

  let el = document.getElementById(STYLE_ID) as HTMLStyleElement | null
  if (!el) {
    el = document.createElement('style')
    el.id = STYLE_ID
    document.head.appendChild(el)
  }
  el.textContent = buildCss(a)

  const hero = a?.sections?.hero
  appearanceText.heroTitle = (hero?.title ?? '').trim()
  appearanceText.heroSubtitle = (hero?.subtitle ?? '').trim()
  appearanceText.heroShowSubtitle = hero?.showSubtitle !== false
  appearanceText.brand = (a?.sections?.header?.brand ?? '').trim()

  setTextReplacements(a?.texts)
  setElementOverrides(a?.overrides)
}

// ---- element overrides (picked element -> text / hide) ----------------

let ovRules: ElementOverride[] = []
let ovObserver: MutationObserver | null = null
let ovScheduled = false

function ovApplyAll(): void {
  ovScheduled = false
  if (typeof document === 'undefined' || !ovRules.length) return
  for (const o of ovRules) {
    if (!o.setText || !o.selector) continue
    let els: NodeListOf<Element>
    try {
      els = document.querySelectorAll(o.selector)
    } catch {
      continue
    }
    els.forEach((el) => {
      if (el.closest('.cm-editor, textarea, input, code, pre, [data-no-fa]')) return
      if (el.querySelector('*')) return // leaf elements only — never nuke a subtree
      const next = o.text ?? ''
      if (el.textContent !== next) el.textContent = next
    })
  }
}

function ovSchedule(): void {
  if (ovScheduled) return
  ovScheduled = true
  requestAnimationFrame(ovApplyAll)
}

/** Set the picked-element overrides (text + hide) and (re)scan. */
export function setElementOverrides(list?: ElementOverride[]): void {
  ovRules = (list || []).filter((o) => o && o.selector)
  if (typeof document === 'undefined' || !document.body) return

  ovApplyAll()

  if (ovRules.some((o) => o.setText) && !ovObserver) {
    ovObserver = new MutationObserver(ovSchedule)
    ovObserver.observe(document.body, { subtree: true, childList: true, characterData: true })
  }
}

// ---- text replacements (find -> replace on visible copy) ---------------

const TR_SKIP_TAGS = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEXTAREA', 'INPUT', 'CODE', 'PRE'])
const TR_SKIP_SEL = '.cm-editor, [contenteditable="true"], [data-no-fa]'

let trRules: TextReplacement[] = []
let trObserver: MutationObserver | null = null

function trSkip(el: Element | null): boolean {
  for (let n: Element | null = el; n; n = n.parentElement) {
    if (TR_SKIP_TAGS.has(n.tagName)) return true
    if (n.matches && n.matches(TR_SKIP_SEL)) return true
  }
  return false
}

function trApply(node: Text): void {
  const orig = node.nodeValue
  if (!orig || !trRules.length) return
  let v = orig
  for (const r of trRules) {
    if (!r.find) continue
    if (r.mode === 'contains') {
      if (r.replace.includes(r.find)) continue // guard against an endless loop
      if (v.includes(r.find)) v = v.split(r.find).join(r.replace)
    } else if (v.trim() === r.find) {
      const lead = /^\s*/.exec(v)?.[0] ?? ''
      const trail = /\s*$/.exec(v)?.[0] ?? ''
      v = lead + r.replace + trail
    }
  }
  if (v !== orig && !trSkip(node.parentElement)) node.nodeValue = v
}

function trWalk(root: Node): void {
  const tw = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
  const nodes: Text[] = []
  for (let n = tw.nextNode(); n; n = tw.nextNode()) nodes.push(n as Text)
  nodes.forEach(trApply)
}

/** Set the active find→replace rules and (re)scan the page. */
export function setTextReplacements(list?: TextReplacement[]): void {
  trRules = (list || []).filter((r) => r && r.find)
  if (typeof document === 'undefined' || !document.body) return

  trWalk(document.body)

  if (trRules.length && !trObserver) {
    trObserver = new MutationObserver((muts) => {
      for (const m of muts) {
        if (m.type === 'characterData') {
          if (m.target.nodeType === Node.TEXT_NODE) trApply(m.target as Text)
        } else {
          m.addedNodes.forEach((n) => {
            if (n.nodeType === Node.TEXT_NODE) trApply(n as Text)
            else if (n.nodeType === Node.ELEMENT_NODE) trWalk(n)
          })
        }
      }
    })
    trObserver.observe(document.body, { subtree: true, childList: true, characterData: true })
  }
}

// ---- element picker (preview iframe only) -----------------------------

function stableClass(c: string): boolean {
  if (!c || c.length > 40) return false
  if (/^(is-|has-|router-link|fade-|slide-|v-|ant-motion|ant-fade|ant-zoom|ant-slide|ant-move)/.test(c))
    return false
  if (/[0-9]{3,}/.test(c)) return false
  return true
}

/** Shortest stable CSS path that is unique in the document. */
function cssPath(el: Element): string {
  if (el instanceof HTMLElement && el.id && /^[A-Za-z][\w-]*$/.test(el.id)) return '#' + el.id
  const parts: string[] = []
  let node: Element | null = el
  let depth = 0
  while (node && node.nodeType === 1 && node !== document.documentElement && depth < 6) {
    let seg = node.tagName.toLowerCase()
    const cls = Array.from(node.classList).filter(stableClass).slice(0, 3)
    if (cls.length) {
      seg += '.' + cls.map((c) => CSS.escape(c)).join('.')
    }
    const parent: Element | null = node.parentElement
    if (parent) {
      const twins = Array.from(parent.children).filter((c) => c.tagName === node!.tagName)
      if (twins.length > 1) seg += `:nth-of-type(${twins.indexOf(node) + 1})`
    }
    parts.unshift(seg)
    const sel = parts.join(' > ')
    try {
      if (document.querySelectorAll(sel).length === 1) return sel
    } catch {
      /* invalid mid-build — keep climbing */
    }
    node = parent
    depth++
  }
  return parts.join(' > ')
}

let picking = false
let hoverEl: HTMLElement | null = null

function clearHover(): void {
  if (!hoverEl) return
  hoverEl.style.removeProperty('outline')
  hoverEl.style.removeProperty('outline-offset')
  hoverEl = null
}

function onPickOver(e: Event): void {
  const t = e.target as HTMLElement
  if (!t || t === document.body || t === document.documentElement) return
  clearHover()
  hoverEl = t
  t.style.setProperty('outline', '2px solid #6366f1', 'important')
  t.style.setProperty('outline-offset', '-2px', 'important')
}

function onPickClick(e: MouseEvent): void {
  e.preventDefault()
  e.stopImmediatePropagation()
  const t = e.target as Element
  if (!t) return
  const isLeaf = !t.querySelector('*')
  let baseFontSize = ''
  try {
    baseFontSize = getComputedStyle(t).fontSize || ''
  } catch {
    /* detached */
  }
  window.parent?.postMessage(
    {
      __dotin: 'picked',
      selector: cssPath(t),
      text: isLeaf ? (t.textContent || '').trim() : '',
      label: (t.textContent || t.tagName).trim().slice(0, 60),
      isLeaf,
      baseFontSize,
    },
    location.origin,
  )
}

function setPickMode(on: boolean): void {
  if (on === picking) return
  picking = on
  if (on) {
    document.addEventListener('mouseover', onPickOver, true)
    document.addEventListener('click', onPickClick, true)
    document.documentElement.style.cursor = 'crosshair'
  } else {
    document.removeEventListener('mouseover', onPickOver, true)
    document.removeEventListener('click', onPickClick, true)
    document.documentElement.style.removeProperty('cursor')
    clearHover()
  }
}

// ---- preview bridge -----------------------------------------------------

let previewLive = false
/** True once the admin iframe has pushed at least one draft — the 1s config
 *  poll then stops re-applying so it can't fight the live edits. */
export function isPreviewLive(): boolean {
  return isPreview && previewLive
}

/** Wire up the preview channel (no-op outside an admin preview iframe). */
export function initAppearanceBridge(): void {
  if (typeof window === 'undefined' || !isPreview) return
  document.documentElement.dataset.preview = '1'

  window.addEventListener('message', (e: MessageEvent) => {
    if (e.origin !== location.origin) return
    const d = e.data
    if (!d || typeof d.__dotin !== 'string') return
    if (d.__dotin === 'appearance') {
      previewLive = true
      applyAppearance(d.payload as Appearance)
    } else if (d.__dotin === 'pick') {
      setPickMode(!!d.on)
    }
  })

  try {
    window.parent?.postMessage({ __dotin: 'appearance-ready' }, location.origin)
  } catch {
    /* not framed */
  }
}
