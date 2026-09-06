// App-wide Persian digits. Walks the rendered DOM and rewrites ASCII digits in
// visible text (and placeholders) to Persian ones, then keeps doing it for
// anything Vue / Ant Design renders later — including the dropdowns, tooltips
// and toasts Ant Design portals onto <body> outside #app.
//
// Deliberately left ASCII: the Java code editor, <code>/<pre> blocks, and form
// fields the user types into — Persian digits there would corrupt submissions.
// Opt an element out explicitly with `data-no-fa` (applies to its subtree).

import { toFa } from './fa'

// A <textarea>'s child text node IS its value, so it must stay ASCII; <input>
// is void (no text children) and only reached via the placeholder path, which
// is display-only — so it is deliberately NOT skipped.
const SKIP_TAGS = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEXTAREA', 'CODE', 'PRE', 'KBD', 'SAMP'])

// Any ancestor matching this opts the whole subtree out.
const SKIP_SELECTOR = '.mono, .cm-editor, [contenteditable=""], [contenteditable="true"], [data-no-fa]'

const HAS_ASCII_DIGIT = /[0-9]/

function inSkippedSubtree(el: Element | null): boolean {
  for (let n: Element | null = el; n; n = n.parentElement) {
    if (SKIP_TAGS.has(n.tagName)) return true
    if (n.matches && n.matches(SKIP_SELECTOR)) return true
  }
  return false
}

function convertTextNode(node: Text): void {
  const v = node.nodeValue
  if (!v || !HAS_ASCII_DIGIT.test(v)) return
  if (inSkippedSubtree(node.parentElement)) return
  const next = toFa(v)
  if (next !== v) node.nodeValue = next
}

function convertPlaceholder(el: Element): void {
  const ph = el.getAttribute('placeholder')
  if (!ph || !HAS_ASCII_DIGIT.test(ph)) return
  if (inSkippedSubtree(el)) return
  const next = toFa(ph)
  if (next !== ph) el.setAttribute('placeholder', next)
}

function walk(root: Node): void {
  // Collect first, mutate after — editing nodeValue during the walk is fine
  // here, but a snapshot keeps it obviously safe.
  const tw = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
  const nodes: Text[] = []
  for (let n = tw.nextNode(); n; n = tw.nextNode()) nodes.push(n as Text)
  nodes.forEach(convertTextNode)

  if (root instanceof Element || root instanceof DocumentFragment) {
    if (root instanceof Element && root.hasAttribute('placeholder')) convertPlaceholder(root)
    root.querySelectorAll('[placeholder]').forEach(convertPlaceholder)
  }
}

/** Start converting under `root` (default: <body>, so Ant Design portals count). */
export function startFaDigits(root: Node = document.body): void {
  walk(root)

  const obs = new MutationObserver((mutations) => {
    for (const m of mutations) {
      if (m.type === 'characterData') {
        if (m.target.nodeType === Node.TEXT_NODE) convertTextNode(m.target as Text)
      } else if (m.type === 'attributes') {
        if (m.target instanceof Element) convertPlaceholder(m.target)
      } else {
        m.addedNodes.forEach((n) => {
          if (n.nodeType === Node.TEXT_NODE) convertTextNode(n as Text)
          else if (n.nodeType === Node.ELEMENT_NODE) walk(n)
        })
      }
    }
  })

  obs.observe(root, {
    subtree: true,
    childList: true,
    characterData: true,
    attributes: true,
    attributeFilter: ['placeholder'],
  })
}
