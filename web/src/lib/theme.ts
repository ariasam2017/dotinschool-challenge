// Three-state theme (system / light / dark), persisted in localStorage, plus the
// matching Ant Design Vue token/algorithm bundle. One brand emerald (#006839)
// across both panels; admin adds the compact algorithm for denser controls.

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

// One brand emerald across both panels. Light keeps the exact #006839; dark
// seeds Ant Design a touch brighter so its derived text/hover tones stay
// legible on the near-black ground (the CSS tokens do the same via
// --primary / --primary-bright).
const PRIMARY = {
  user: { light: '#006839', dark: '#0a7d4b' },
  admin: { light: '#006839', dark: '#0a7d4b' },
} as const

/** Ant Design Vue <a-config-provider :theme="antdTheme"> value. */
export const antdTheme = computed(() => {
  const dark = isDark.value
  const admin = APP_MODE === 'admin'
  const primary = PRIMARY[APP_MODE][dark ? 'dark' : 'light']
  const algorithm = dark ? [antTheme.darkAlgorithm] : [antTheme.defaultAlgorithm]
  if (admin) algorithm.push(antTheme.compactAlgorithm)

  // Dark surfaces: the admin panel uses a neutral "Sanaei / 3x-ui" grey stack;
  // the user panel keeps the emerald-black "tech" dark. Brand green is the same
  // in both (kept exact at #006839, seeded a touch brighter for Ant's derived
  // tones — see PRIMARY above).
  const darkBg = admin ? '#141414' : '#0a1310'
  const darkText = admin ? '#ededed' : '#eef3f0'
  const darkBorder = admin ? 'rgba(255,255,255,0.13)' : 'rgba(150,228,190,0.14)'
  const darkBorder2 = admin ? 'rgba(255,255,255,0.07)' : 'rgba(150,228,190,0.07)'

  return {
    algorithm,
    token: {
      colorPrimary: primary,
      colorInfo: dark ? '#56a9f5' : '#1d4ed8',
      colorSuccess: dark ? '#3ccf8d' : '#006839',
      colorWarning: dark ? '#edb32a' : '#ca8a04',
      colorError: dark ? '#f2565b' : '#dc2626',
      borderRadius: 10,
      wireframe: false,
      fontFamily: "'IRANYekanX', 'Inter', system-ui, -apple-system, 'Segoe UI', sans-serif",
      colorBgBase: dark ? darkBg : '#ffffff',
      colorBgLayout: dark ? darkBg : '#f8fafc',
      colorTextBase: dark ? darkText : '#0f172a',
      colorBorder: dark ? darkBorder : '#e2e8f0',
      colorBorderSecondary: dark ? darkBorder2 : '#eef2f7',
    },
  }
})
