// i18n skeleton — Persian is the default and the only complete locale. English
// is an optional stub kept in sync key-for-key (the x-ui translation-file
// pattern). Components currently carry Persian strings inline; new strings
// should be added here and pulled with `t()` so a full extraction is a
// mechanical follow-up rather than a redesign.

import { ref, computed } from 'vue'
import { fa } from './fa'
import { en } from './en'

export type Locale = 'fa' | 'en'
export type MessageTree = { [k: string]: string | MessageTree }

const MESSAGES: Record<Locale, MessageTree> = { fa, en }
const STORAGE_KEY = 'dotinschool_locale_v1'

function detect(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'fa' || saved === 'en') return saved
  } catch {
    /* ignore */
  }
  return navigator.language?.toLowerCase().startsWith('en') ? 'en' : 'fa'
}

export const locale = ref<Locale>(detect())
export const isRTL = computed(() => locale.value === 'fa')

export function setLocale(l: Locale): void {
  locale.value = l
  try {
    localStorage.setItem(STORAGE_KEY, l)
  } catch {
    /* ignore */
  }
  document.documentElement.setAttribute('lang', l)
  document.documentElement.setAttribute('dir', l === 'fa' ? 'rtl' : 'ltr')
}

function resolve(tree: MessageTree, path: string[]): string | undefined {
  let node: string | MessageTree | undefined = tree
  for (const key of path) {
    if (typeof node !== 'object' || node === null) return undefined
    node = node[key]
  }
  return typeof node === 'string' ? node : undefined
}

/** t('nav.overview') with {name} interpolation via the second arg. */
export function t(key: string, params?: Record<string, string | number>): string {
  const path = key.split('.')
  let out = resolve(MESSAGES[locale.value], path) ?? resolve(MESSAGES.fa, path) ?? key
  if (params) {
    for (const [k, v] of Object.entries(params)) out = out.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v))
  }
  return out
}

export function useI18n() {
  return { t, locale, setLocale, isRTL }
}
