// Three-state theme (system / light / dark), persisted in localStorage, plus the
// matching Ant Design Vue token/algorithm bundle. The panel personality
// (user = green, admin = indigo) is fixed per entry point.

import { ref, computed, watch } from 'vue'
import { theme as antTheme } from 'ant-design-vue'
import { APP_MODE } from './mode'

export type ThemePref = 'system' | 'light' | 'dark'
const STORAGE_KEY = 'dotinschool_theme_v2'

function readPref(): ThemePref {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch {
    /* private mode / blocked storage */
  }
  return 'system'
}

export const themePref = ref<ThemePref>(readPref())

const media = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(media.matches)
media.addEventListener('change', (e) => (systemDark.value = e.matches))

export const isDark = computed(
  () => themePref.value === 'dark' || (themePref.value === 'system' && systemDark.value),
)

export function applyTheme(): void {
  const el = document.documentElement
  if (themePref.value === 'system') el.removeAttribute('data-theme')
  else el.setAttribute('data-theme', themePref.value)
  el.setAttribute('data-panel', APP_MODE)
  el.setAttribute('lang', 'fa')
  el.setAttribute('dir', 'rtl')
}

export function setTheme(p: ThemePref): void {
  themePref.value = p
}

/** system -> light -> dark -> system */
export function cycleTheme(): ThemePref {
  const order: ThemePref[] = ['system', 'light', 'dark']
  const next = order[(order.indexOf(themePref.value) + 1) % order.length]
  themePref.value = next
  return next
}

watch(
  themePref,
  (p) => {
    try {
      localStorage.setItem(STORAGE_KEY, p)
    } catch {
      /* ignore */
    }
    applyTheme()
  },
  { immediate: true },
)

const PRIMARY = {
  user: { light: '#16a34a', dark: '#22c55e' },
  admin: { light: '#4f46e5', dark: '#6366f1' },
} as const

/** Ant Design Vue <a-config-provider :theme="antdTheme"> value. */
export const antdTheme = computed(() => {
  const dark = isDark.value
  const primary = PRIMARY[APP_MODE][dark ? 'dark' : 'light']
  const algorithm = dark ? [antTheme.darkAlgorithm] : [antTheme.defaultAlgorithm]
  if (APP_MODE === 'admin') algorithm.push(antTheme.compactAlgorithm)
  return {
    algorithm,
    token: {
      colorPrimary: primary,
      colorInfo: primary,
      colorSuccess: dark ? '#22c55e' : '#16a34a',
      colorWarning: dark ? '#eab308' : '#ca8a04',
      colorError: dark ? '#ef4444' : '#dc2626',
      borderRadius: 10,
      wireframe: false,
      fontFamily: "'IRANYekanX', 'Inter', system-ui, -apple-system, 'Segoe UI', sans-serif",
      colorBgBase: dark ? '#0b1220' : '#ffffff',
      colorBgLayout: dark ? '#0b1220' : '#f8fafc',
      colorTextBase: dark ? '#f8fafc' : '#0f172a',
      colorBorder: dark ? 'rgba(255,255,255,0.12)' : '#e2e8f0',
      colorBorderSecondary: dark ? 'rgba(255,255,255,0.07)' : '#eef2f7',
    },
  }
})
