<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { ApiError, getConfig, getVersion } from '@/api/client'
import AppIcon from '@/components/AppIcon.vue'

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
  <!-- One screen, permanently dark console chrome (this login does not
       answer the light/dark toggle — the toggle lives inside the dashboard
       itself, once the admin is in). -->
  <div class="console-page">
    <header class="page-top">
      <span class="brand-mark"><AppIcon name="shield" :size="18" /></span>
      <span class="brand-word">داتین‌اسکول</span>
    </header>

    <main class="page-center">
      <h1 class="page-title">ورود به سامانهٔ مدیریت چالش</h1>

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
    </main>

    <footer class="page-foot">
      <a class="page-back" href="/">
        <AppIcon name="chevron-right" :size="13" />
        بازگشت به سامانهٔ چالش
      </a>
      <span v-if="version" class="page-version">نسخهٔ {{ version }}</span>
    </footer>
  </div>
</template>

<style scoped>
/* Fixed-dark console chrome for the whole screen — not theme tokens, on
   purpose: the login to an ops console reads the same regardless of the
   visitor's OS light/dark preference, like a terminal. */
.console-page {
  --cbg: #0a1310;
  --cline: rgba(150, 228, 190, 0.14);
  --ctext: #eef3f0;
  --ctext-dim: #a2b7ab;
  --ctext-faint: #6f847a;
  --cbright: #3ccf8d;
  position: relative;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  background: var(--cbg);
  color: var(--ctext);
  overflow: hidden;
}
/* structural schematic grid across the whole page — console chrome, not
   decoration for its own sake. */
.console-page::before {
  content: '';
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(var(--cline) 1px, transparent 1px),
    linear-gradient(90deg, var(--cline) 1px, transparent 1px);
  background-size: 44px 44px;
  -webkit-mask-image: radial-gradient(85% 70% at 50% 38%, #000, transparent 88%);
  mask-image: radial-gradient(85% 70% at 50% 38%, #000, transparent 88%);
  opacity: 0.6;
  pointer-events: none;
}

.page-top {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: var(--space-5);
}
.brand-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: var(--primary);
  color: var(--on-primary);
  flex: none;
}
.brand-word {
  font-weight: 800;
  font-size: 0.98rem;
  letter-spacing: -0.01em;
}

.page-center {
  position: relative;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-5);
}
.page-title {
  margin: 0 0 var(--space-6);
  font-size: clamp(1.5rem, 4vw, 2rem);
  font-weight: 800;
  line-height: 1.4;
  letter-spacing: -0.01em;
  text-align: center;
  color: var(--ctext);
}

.form-alerts {
  width: 100%;
  max-width: 380px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: var(--space-4);
}
form.stack {
  width: 100%;
  max-width: 380px;
  gap: var(--space-4);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.field-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--ctext-dim);
}
.eye {
  border: none;
  background: transparent;
  color: var(--ctext-faint);
  display: grid;
  place-items: center;
}
.page-center :deep(.ant-input),
.page-center :deep(.ant-input-affix-wrapper) {
  background: #101914;
  border: 1.5px solid var(--cline);
  border-radius: var(--radius-sm);
  color: var(--ctext);
}
/* The password field nests a plain .ant-input inside .ant-input-affix-wrapper
   for the eye-icon suffix. Without this, the inner input keeps its own border
   + radius too, so two rounded rects overlap and the field reads as a pill
   instead of matching the plain username field exactly. */
.page-center :deep(.ant-input-affix-wrapper > .ant-input) {
  background: transparent;
  border: 0;
  border-radius: 0;
  box-shadow: none;
  padding: 0;
}
.page-center :deep(.ant-input::placeholder) {
  color: var(--ctext-faint);
}
.page-center :deep(.ant-input:focus),
.page-center :deep(.ant-input-focused),
.page-center :deep(.ant-input-affix-wrapper-focused) {
  border-color: var(--cbright);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--cbright) 30%, transparent);
}
.page-center :deep(.ant-btn):active {
  transform: translateY(1px);
}

.page-foot {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  flex-wrap: wrap;
  padding: var(--space-5);
}
.page-back {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--ctext-dim);
  transition: color var(--dur) var(--ease);
}
.page-back:hover {
  color: var(--cbright);
}
.page-version {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: var(--ctext-faint);
  font-variant-numeric: tabular-nums;
}

/* one orchestrated entrance moment; nothing else on the page animates */
@media (prefers-reduced-motion: no-preference) {
  .page-title,
  form.stack {
    animation: page-rise 0.5s var(--ease) both;
  }
  form.stack {
    animation-delay: 0.08s;
  }
}
@keyframes page-rise {
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
