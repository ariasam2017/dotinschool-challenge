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

const FS_RE = /^[0-9]{1,3}(\.[0-9]{1,2})?(px|rem|em|%)$/
const FW_RE = /^(300|400|500|600|700|800|900|normal|bold)$/
const TA_RE = /^(right|left|center|justify)$/

// typography that should also cascade to the text-bearing children of the
// picked element (so setting font-size on a wrapper still resizes its heading)
const TEXT_DESCENDANTS =
  ':where(h1,h2,h3,h4,h5,h6,p,a,span,strong,em,b,i,small,label,li,td,th,dt,dd,figcaption,blockquote)'

interface BagOut {
  self: string[] // everything, applied to the selector
  text: string[] // typography only, applied to selector + text descendants
}

/** CSS declarations for one style bag (unvalidated keys are dropped). */
function bagDecls(raw?: unknown): BagOut {
  const b = (raw || {}) as Record<string, string | undefined>
  const self: string[] = []
  const text: string[] = []
  const typo = (v: string) => {
    self.push(v)
    text.push(v)
  }
  if (b.fontSize && FS_RE.test(b.fontSize)) {
    typo(`font-size:${b.fontSize}`)
    // resized text must be free to wrap, or a bigger size just overflows
    typo('white-space:normal')
    typo('overflow-wrap:break-word')
  }
  if (b.fontWeight && FW_RE.test(String(b.fontWeight))) typo(`font-weight:${b.fontWeight}`)
  if (b.textAlign && TA_RE.test(b.textAlign)) typo(`text-align:${b.textAlign}`)
  const c = safeValue(b.color)
  if (c) typo(`color:${c}`)

  const push = (prop: string, v?: string) => {
    const s = safeValue(v)
    if (s) self.push(`${prop}:${s}`)
  }
  push('background', b.background)
  push('padding', b.padding)
  push('margin', b.margin)
  if (safeValue(b.borderWidth)) self.push(`border-width:${safeValue(b.borderWidth)}`, 'border-style:solid')
  push('border-color', b.borderColor)
  push('border-radius', b.borderRadius)
  push('width', b.width)
  push('max-width', b.maxWidth)
  return { self, text }
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

  const MOBILE_Q = '(max-width: 767.98px)'
  const DESKTOP_Q = '(min-width: 768px)'
  for (const o of a.overrides || []) {
    if (!o.selector || o.selector.length > 400 || !SELECTOR_RE.test(o.selector)) continue
    const sel = o.selector

    const legacyAll: Record<string, string | undefined> = {
      fontSize: o.fontSize, fontWeight: o.fontWeight, textAlign: o.textAlign,
      color: o.color, background: o.background, padding: o.padding, margin: o.margin,
      borderWidth: o.borderWidth, borderColor: o.borderColor, borderRadius: o.borderRadius,
      width: o.width, maxWidth: o.maxWidth,
    }
    const all = o.all && Object.keys(o.all).length ? o.all : legacyAll
    const imp = (decls: string[]) => decls.map((x) => x + ' !important').join(';')
    // typography also lands on the picked element's text children, so setting
    // e.g. font-size on a wrapper still resizes the heading inside it
    const rule = (out2: BagOut): string => {
      const parts: string[] = []
      if (out2.self.length) parts.push(`${sel}{${imp(out2.self)}}`)
      if (out2.text.length) parts.push(`${sel} ${TEXT_DESCENDANTS}{${imp(out2.text)}}`)
      return parts.join('')
    }
    const wrap = (q: string, css: string) => (css ? `@media ${q}{${css}}` : '')

    out.push(rule(bagDecls(all)))
    out.push(wrap(MOBILE_Q, rule(bagDecls(o.mobile))))
    out.push(wrap(DESKTOP_Q, rule(bagDecls(o.desktop))))

    if (o.hidden) out.push(`${sel}{display:none !important}`)
    if (o.hiddenMobile) out.push(`@media ${MOBILE_Q}{${sel}{display:none !important}}`)
    if (o.hiddenDesktop) out.push(`@media ${DESKTOP_Q}{${sel}{display:none !important}}`)
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
let ovMql: MediaQueryList | null = null

/** The text for the current viewport: the موبایل / دسکتاپ layer if set,
 *  otherwise the every-viewport text. */
function ovTextFor(o: ElementOverride): string {
  const mobile = ovMql ? ovMql.matches : false
  const scoped = mobile ? o.textMobile : o.textDesktop
  return (scoped && scoped.trim() ? scoped : o.text) ?? ''
}

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
    const next = ovTextFor(o)
    els.forEach((el) => {
      if (el.closest('.cm-editor, textarea, input, code, pre, [data-no-fa]')) return
      // if the picked element wraps its text, drill into the text-bearing child
      const target = el.querySelector('*') ? textLeaf(el) : el
      if (target.querySelector('*')) return // still branchy — never nuke a subtree
      if (target.textContent !== next) target.textContent = next
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

  if (!ovMql && typeof window !== 'undefined') {
    ovMql = window.matchMedia('(max-width: 767.98px)')
    ovMql.addEventListener('change', ovSchedule) // reapply on breakpoint crossing
  }

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

/** Own direct non-whitespace text? */
function hasOwnText(el: Element): boolean {
  return Array.from(el.childNodes).some((n) => n.nodeType === 3 && (n.nodeValue || '').trim())
}

/** Descend to the innermost element that actually carries the visible text,
 *  so clicking near a heading lands on the heading, not its wrapper. */
function textLeaf(el: Element): Element {
  let cur = el
  for (let i = 0; i < 4; i++) {
    if (hasOwnText(cur)) return cur
    if (cur.children.length === 1) {
      cur = cur.children[0]
      continue
    }
    break
  }
  return el
}

function describe(el: Element): Record<string, unknown> {
  let baseFontSize = ''
  try {
    baseFontSize = getComputedStyle(el).fontSize || ''
  } catch {
    /* detached */
  }
  const isLeaf = !el.querySelector('*')
  return {
    selector: cssPath(el),
    text: isLeaf ? (el.textContent || '').trim() : '',
    label: (el.textContent || el.tagName).trim().slice(0, 60),
    isLeaf,
    baseFontSize,
  }
}

function onPickClick(e: MouseEvent): void {
  e.preventDefault()
  e.stopImmediatePropagation()
  const raw = e.target as Element
  if (!raw) return
  const el = textLeaf(raw)
  window.parent?.postMessage({ __dotin: 'picked', ...describe(el) }, location.origin)
}

/** Move the current selection to its parent / first child and report back. */
function onRetarget(selector: string, dir: 'up' | 'down'): void {
  let el: Element | null = null
  try {
    el = document.querySelector(selector)
  } catch {
    return
  }
  if (!el) return
  const next = dir === 'up' ? el.parentElement : el.firstElementChild
  if (!next || next === document.body || next === document.documentElement) return
  window.parent?.postMessage(
    { __dotin: 'retargeted', from: selector, ...describe(next) },
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
    } else if (d.__dotin === 'retarget' && typeof d.selector === 'string') {
      onRetarget(d.selector, d.dir === 'up' ? 'up' : 'down')
    }
  })

  try {
    window.parent?.postMessage({ __dotin: 'appearance-ready' }, location.origin)
  } catch {
    /* not framed */
  }
}
