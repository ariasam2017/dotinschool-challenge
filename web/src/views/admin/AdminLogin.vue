<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { ApiError, getConfig } from '@/api/client'
import AppIcon from '@/components/AppIcon.vue'

const store = useAdminStore()
const form = reactive({ username: 'administrator', password: '' })
const showPass = ref(false)
const loading = ref(false)
const error = ref('')
const reachable = ref<boolean | null>(null)

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
  <!-- Linear / Vercel register: near-black, neutral hairlines, no card box,
     one accent colour (the brand green) and nothing else competing for
     attention. Permanently dark, independent of the light/dark toggle. -->
  <div class="dev-shell">
    <div class="dev-glow" aria-hidden="true" />

    <main class="dev-panel">
      <div class="dev-brand">
        <span class="dev-mark"><AppIcon name="shield" :size="19" /></span>
      </div>

      <h1 class="dev-title">ورود به پنل مدیریت</h1>
      <p class="dev-sub">برای دسترسی به کنسول مدیریت وارد شو</p>

      <div class="dev-alerts">
        <a-alert
          v-if="reachable === false"
          type="error"
          show-icon
          message="سرور در دسترس نیست"
          description="این صفحه بارگذاری شده ولی درخواست‌های API به سرور نمی‌رسند. معمولا یکی از این‌هاست: فیلترشکن/پروکسی سیستم (v2rayN و…) که ترافیک localhost را هم رد می‌کند — آن را روی «Bypass» برای آدرس‌های محلی بگذارید یا موقتا خاموش کنید؛ یا افزونه ad-blocker روی این صفحه؛ یا اینکه پنل را روی پورت اشتباه باز کرده‌اید (باید همان پورتی باشد که کانتینر منتشر کرده). بعد Ctrl+Shift+R."
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
          <a-input v-model:value="form.username" dir="ltr" autocomplete="username" />
        </label>
        <label class="field">
          <span class="field-label">رمز عبور</span>
          <a-input
            v-model:value="form.password"
            :type="showPass ? 'text' : 'password'"
            dir="ltr"
            autocomplete="current-password"
          >
            <template #suffix>
              <button type="button" class="eye" :aria-label="showPass ? 'پنهان‌کردن رمز' : 'نمایش رمز'" @click="showPass = !showPass">
                <AppIcon :name="showPass ? 'eye-off' : 'eye'" :size="15" />
              </button>
            </template>
          </a-input>
        </label>
        <a-button type="primary" block html-type="submit" :loading="loading">
          ورود
        </a-button>
      </form>

      <div class="dev-foot">
        <a class="dev-back" href="/">بازگشت به سامانه چالش</a>
      </div>
    </main>
  </div>
</template>

<style scoped>
.dev-shell {
  --bg: #0a0a0b;
  --line: rgba(255, 255, 255, 0.1);
  --line-soft: rgba(255, 255, 255, 0.06);
  --text: #f2f2f3;
  --text-dim: #8b8b8f;
  --text-faint: #5c5c60;
  --field-bg: rgba(255, 255, 255, 0.03);
  /* locked brand: #006839 for solid fills, its luminous dark-mode sibling
     #3ccf8d (same as tokens.css's --primary-bright) for rings/links, where
     the flat fill alone would be too low-contrast on near-black. */
  --brand: #006839;
  --brand-strong: #0a7d4b;
  --brand-bright: #3ccf8d;
  position: relative;
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-5);
  background: var(--bg);
  color: var(--text);
  overflow: hidden;
}
/* one soft, colourless light source above the panel — the only "atmosphere"
   this screen allows itself. No grid, no gradient wash, no second colour. */
.dev-glow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(60rem 36rem at 50% -8%, rgba(255, 255, 255, 0.07), transparent 70%);
}

.dev-panel {
  position: relative;
  width: 100%;
  max-width: 408px;
}
.dev-brand {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 32px;
}
.dev-mark {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--brand);
  color: #fff;
  flex: none;
}
.dev-title {
  margin: 0 0 8px;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: -0.01em;
  line-height: 1.4;
  text-align: center;
  color: var(--text);
}
.dev-sub {
  margin: 0 0 36px;
  font-size: 14.5px;
  line-height: 1.7;
  text-align: center;
  color: var(--text-dim);
}
.dev-alerts {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 22px;
}
.stack {
  gap: 17px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.field-label {
  font-size: 13.7px;
  font-weight: 500;
  color: var(--text-dim);
}
.eye {
  border: none;
  background: transparent;
  color: var(--text-faint);
  display: grid;
  place-items: center;
}

.dev-panel :deep(.ant-input),
.dev-panel :deep(.ant-input-affix-wrapper) {
  background: var(--field-bg);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--text);
  height: 46px;
  font-size: 15.5px;
}
.dev-panel :deep(.ant-input-affix-wrapper > .ant-input) {
  background: transparent;
  border: 0;
  height: auto;
}
.dev-panel :deep(.ant-input::placeholder) {
  color: var(--text-faint);
}
.dev-panel :deep(.ant-input:hover),
.dev-panel :deep(.ant-input-affix-wrapper:hover) {
  border-color: rgba(255, 255, 255, 0.18);
}
.dev-panel :deep(.ant-input:focus),
.dev-panel :deep(.ant-input-focused),
.dev-panel :deep(.ant-input-affix-wrapper-focused) {
  border-color: var(--brand-bright);
  box-shadow: 0 0 0 3px rgba(60, 207, 141, 0.18);
}
.dev-panel :deep(.ant-btn-primary) {
  height: 46px;
  background: var(--brand);
  border-color: var(--brand);
  border-radius: 8px;
  font-size: 15.5px;
  font-weight: 600;
}
.dev-panel :deep(.ant-btn-primary:not(:disabled):hover) {
  background: var(--brand-strong);
  border-color: var(--brand-strong);
}
.dev-panel :deep(.ant-btn):active {
  transform: translateY(1px);
}

.dev-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 28px;
  padding-top: 22px;
  border-top: 1px solid var(--line-soft);
}
.dev-back {
  font-size: 13.7px;
  font-weight: 500;
  color: var(--text-dim);
  transition: color var(--dur) var(--ease);
}
.dev-back:hover {
  color: var(--text);
}

@media (prefers-reduced-motion: no-preference) {
  .dev-panel {
    animation: dev-rise 0.4s var(--ease) both;
  }
}
@keyframes dev-rise {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
</style>
