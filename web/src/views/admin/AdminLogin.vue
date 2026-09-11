<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { ApiError, getConfig, getVersion } from '@/api/client'
import AppIcon from '@/components/AppIcon.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

const store = useAdminStore()
const form = reactive({ username: 'administrator', password: '' })
const showPass = ref(false)
const loading = ref(false)
const error = ref('')
const reachable = ref<boolean | null>(null)
const version = ref('')

// Proactive connectivity probe: GET /api/config is public. If it fails the
// problem is the network/proxy/extension, not the password — say so up front so
// a wrong "ورود ناموفق" doesn't send the admin chasing the credentials.
onMounted(async () => {
  try {
    await getConfig()
    reachable.value = true
  } catch {
    reachable.value = false
  }
  getVersion()
    .then((r) => (version.value = r.version || ''))
    .catch(() => {})
})

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const ok = await store.login(form.username.trim(), form.password)
    if (ok) reachable.value = true
    else error.value = 'نام کاربری یا رمز عبور اشتباه است.'
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'ورود ناموفق بود'
    if (e instanceof ApiError && e.code === 'NETWORK_ERROR') reachable.value = false
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <!-- The form is the reason this page exists, so it is what an RTL reader
       meets first (this pane is first in DOM order = right-hand side under
       plain `flex-direction: row` in RTL): shortest path from landing to
       "logged in". The console pane is the identity/context, read second. -->
  <div class="console-shell">
    <main class="form-pane">
      <div class="form-top"><ThemeToggle /></div>

      <div class="form-box">
        <p class="form-kicker">ورود مدیر</p>
        <h1 class="form-title">شناسه‌ات را وارد کن</h1>

        <div class="form-alerts">
          <a-alert
            v-if="reachable === false"
            type="error"
            show-icon
            message="سرور در دسترس نیست"
            description="این صفحه بارگذاری شده ولی درخواست‌های API به سرور نمی‌رسند. معمولاً یکی از این‌هاست: فیلترشکن/پروکسی سیستم (v2rayN و…) که ترافیک localhost را هم رد می‌کند — آن را روی «Bypass» برای آدرس‌های محلی بگذارید یا موقتاً خاموش کنید؛ یا افزونهٔ ad-blocker روی این صفحه؛ یا اینکه پنل را روی پورت اشتباه باز کرده‌اید (باید همان پورتی باشد که کانتینر منتشر کرده). بعد Ctrl+Shift+R."
          />
          <a-alert
            v-if="store.sessionExpiredNote"
            type="warning"
            :message="store.sessionExpiredNote"
            show-icon
            banner
          />
          <a-alert v-if="error" type="error" :message="error" show-icon banner />
        </div>

        <form class="stack" @submit.prevent="submit">
          <label class="field">
            <span class="field-label">نام کاربری</span>
            <a-input v-model:value="form.username" size="large" dir="ltr" autocomplete="username" />
          </label>
          <label class="field">
            <span class="field-label">رمز عبور</span>
            <a-input
              v-model:value="form.password"
              :type="showPass ? 'text' : 'password'"
              size="large"
              dir="ltr"
              autocomplete="current-password"
            >
              <template #suffix>
                <button type="button" class="eye" :aria-label="showPass ? 'پنهان‌کردن رمز' : 'نمایش رمز'" @click="showPass = !showPass">
                  <AppIcon :name="showPass ? 'eye-off' : 'eye'" :size="16" />
                </button>
              </template>
            </a-input>
          </label>
          <a-button type="primary" size="large" block html-type="submit" :loading="loading">
            ورود به پنل مدیریت
          </a-button>
        </form>
      </div>
    </main>

    <aside class="console-pane">
      <div class="console-brand">
        <span class="console-mark"><AppIcon name="shield" :size="18" /></span>
        <span class="console-word">داتین‌اسکول</span>
      </div>

      <div class="console-body">
        <h2 class="console-headline">کنسولِ مدیریتِ چالش</h2>
        <p class="console-copy">
          از این‌جا رویداد را زمان‌بندی می‌کنی، چالش را طراحی و وضعیتِ شرکت‌کنندگان را زنده دنبال می‌کنی.
        </p>
        <ul class="console-caps">
          <li><AppIcon name="file-code" :size="14" /><span>طراحی چالش، سطح دشواری و مثال‌ها</span></li>
          <li><AppIcon name="activity" :size="14" /><span>نظارت زندهٔ وضعیت شرکت‌کنندگان</span></li>
          <li><AppIcon name="wand" :size="14" /><span>بررسی هوشمند احتمال تولید کد با AI</span></li>
        </ul>
      </div>

      <div class="console-foot">
        <span v-if="version" class="console-version">نسخهٔ {{ version }}</span>
        <a class="console-back" href="/">
          <AppIcon name="chevron-right" :size="13" />
          بازگشت به سامانهٔ چالش
        </a>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.console-shell {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  background: var(--bg);
}
@media (min-width: 900px) {
  .console-shell {
    flex-direction: row;
  }
}

/* ---- form pane: fully theme-aware — this is the surface people actually
   read and type into, so it must stay legible in both modes. ---- */
.form-pane {
  position: relative;
  flex: 1 1 46%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6) var(--space-5);
}
.form-top {
  position: absolute;
  top: var(--space-5);
  inset-inline-end: var(--space-5);
}
.form-box {
  width: 100%;
  max-width: 380px;
}
.form-kicker {
  margin: 0 0 6px;
  font-size: 0.76rem;
  font-weight: 700;
  color: var(--primary-bright);
}
.form-title {
  margin: 0 0 var(--space-6);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.01em;
  line-height: 1.4;
  color: var(--text);
}
.form-alerts {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: var(--space-4);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.field-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-dim);
}
.eye {
  border: none;
  background: transparent;
  color: var(--text-faint);
  display: grid;
  place-items: center;
}
.form-box :deep(.stack) {
  gap: var(--space-4);
}
.form-box :deep(.ant-input),
.form-box :deep(.ant-input-affix-wrapper) {
  background: var(--surface-2);
  border: 1.5px solid var(--border-strong);
  border-radius: var(--radius-sm);
}
.form-box :deep(.ant-input:focus),
.form-box :deep(.ant-input-focused),
.form-box :deep(.ant-input-affix-wrapper-focused) {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px var(--ring);
}
.form-box :deep(.ant-btn):active {
  transform: translateY(1px);
}

/* ---- console pane: deliberately fixed-dark identity chrome, independent of
   the light/dark toggle — like the always-dark bezel on a piece of hardware.
   It holds no form controls, so it never fights legibility; only the form
   pane above answers the theme toggle. ---- */
.console-pane {
  --cbg: #0a1310;
  --cline: rgba(150, 228, 190, 0.14);
  --ctext: #eef3f0;
  --ctext-dim: #a2b7ab;
  --ctext-faint: #6f847a;
  --cbright: #3ccf8d;
  position: relative;
  flex: 1 1 54%;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: var(--space-6);
  padding: var(--space-6);
  background: var(--cbg);
  color: var(--ctext);
  border-inline-start: 1px solid var(--cline);
  overflow: hidden;
}
/* a structural schematic grid, not decoration for its own sake — it reads as
   console chrome, grounded in the "control room" subject of this screen. */
.console-pane::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(var(--cline) 1px, transparent 1px),
    linear-gradient(90deg, var(--cline) 1px, transparent 1px);
  background-size: 44px 44px;
  -webkit-mask-image: linear-gradient(180deg, #000, transparent 85%);
  mask-image: linear-gradient(180deg, #000, transparent 85%);
  opacity: 0.5;
  pointer-events: none;
}
.console-brand,
.console-body,
.console-foot {
  position: relative;
}
.console-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}
.console-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: var(--primary);
  color: var(--on-primary);
  flex: none;
}
.console-word {
  font-weight: 800;
  font-size: 0.98rem;
  letter-spacing: -0.01em;
}
.console-body {
  max-width: 34ch;
}
.console-headline {
  margin: 0 0 var(--space-3);
  font-size: clamp(1.5rem, 3vw, 2rem);
  font-weight: 900;
  line-height: 1.35;
  letter-spacing: -0.01em;
  color: var(--ctext);
}
.console-copy {
  margin: 0;
  font-size: 0.88rem;
  line-height: 2;
  color: var(--ctext-dim);
}
.console-caps {
  list-style: none;
  margin: var(--space-5) 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
}
.console-caps li {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 0;
  font-size: 0.82rem;
  color: var(--ctext-dim);
  border-top: 1px solid var(--cline);
}
.console-caps li:last-child {
  border-bottom: 1px solid var(--cline);
}
.console-caps li :deep(.app-icon) {
  color: var(--cbright);
  flex: none;
}
.console-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  flex-wrap: wrap;
}
.console-version {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: var(--ctext-faint);
  font-variant-numeric: tabular-nums;
}
.console-back {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--ctext-dim);
  transition: color var(--dur) var(--ease);
}
.console-back:hover {
  color: var(--cbright);
}

/* mobile: console pane drops below the form as a compact strip; the grid
   fades further since it has less room to read as a pattern. */
@media (max-width: 899px) {
  .console-pane {
    flex: none;
    border-inline-start: none;
    border-top: 1px solid var(--cline);
    padding: var(--space-5);
    gap: var(--space-4);
  }
  .console-pane::before {
    opacity: 0.3;
  }
  .console-body {
    max-width: none;
  }
  .console-headline {
    font-size: 1.3rem;
  }
}

/* one orchestrated entrance moment on the form; nothing else on the page
   animates on load. */
@media (prefers-reduced-motion: no-preference) {
  .form-box {
    animation: form-rise 0.5s var(--ease) both;
  }
}
@keyframes form-rise {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
</style>
