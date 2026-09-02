// Which panel to render and the router base — both come from <meta> tags the Go
// server injects into index.html (it knows the secret admin path / sub-domain;
// the SPA does not). A strict CSP blocks inline <script>, so meta tags are the
// channel.

function meta(name: string): string | null {
  return document.querySelector(`meta[name="${name}"]`)?.getAttribute('content') ?? null
}

export type AppMode = 'user' | 'admin'

export const APP_MODE: AppMode = meta('dotin-mode') === 'admin' ? 'admin' : 'user'

/** Router base path, e.g. "/" or "/panel-9f3a7c1d/". Always ends with "/". */
export const ROUTER_BASE: string = (() => {
  let b = meta('dotin-base') || '/'
  if (!b.endsWith('/')) b += '/'
  return b
})()

export const IS_ADMIN = APP_MODE === 'admin'
export const IS_USER = APP_MODE === 'user'
