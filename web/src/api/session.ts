// Browser-side token storage. Keys are kept identical to the original so an
// already-logged-in browser is not signed out by the rewrite.

const ADMIN_TOKEN_KEY = 'dotinschool_admin_token_v1'
const ADMIN_USER_KEY = 'dotinschool_admin_user_v1'
const REMEMBER_SESSION_KEY = 'dotinschool_user_session_v1'

function safe<T>(fn: () => T, fallback: T): T {
  try {
    return fn()
  } catch {
    return fallback
  }
}

/* ---- admin (sessionStorage: cleared when the tab closes) ---- */
export const adminToken = {
  get: () => safe(() => sessionStorage.getItem(ADMIN_TOKEN_KEY), null),
  set: (t: string, user: string) =>
    safe(() => {
      sessionStorage.setItem(ADMIN_TOKEN_KEY, t)
      sessionStorage.setItem(ADMIN_USER_KEY, user)
    }, undefined),
  clear: () =>
    safe(() => {
      sessionStorage.removeItem(ADMIN_TOKEN_KEY)
      sessionStorage.removeItem(ADMIN_USER_KEY)
    }, undefined),
  user: () => safe(() => sessionStorage.getItem(ADMIN_USER_KEY), null),
  isAuthed: () => !!safe(() => sessionStorage.getItem(ADMIN_TOKEN_KEY), null),
}

/* ---- participant "remember this device" (localStorage: the OTP-verified
   session token, never the raw phone number) ---- */
export const rememberedSession = {
  get: () => safe(() => localStorage.getItem(REMEMBER_SESSION_KEY), null),
  set: (t: string) => safe(() => localStorage.setItem(REMEMBER_SESSION_KEY, t), undefined),
  clear: () => safe(() => localStorage.removeItem(REMEMBER_SESSION_KEY), undefined),
}
