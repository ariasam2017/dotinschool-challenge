<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { ApiError, getConfig } from '@/api/client'
import AppIcon from '@/components/AppIcon.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

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
  <div class="login-shell">
    <div class="login-top"><ThemeToggle /></div>
    <div class="login-card card">
      <div class="login-head">
        <span class="brand-mark"><AppIcon name="shield" :size="20" /></span>
        <div>
          <h1>پنل مدیریت dotinschool</h1>
          <p class="faint">برای ادامه وارد شوید</p>
        </div>
      </div>

      <a-alert
        v-if="reachable === false"
        type="error"
        show-icon
        style="margin-bottom: 14px"
        message="سرور در دسترس نیست"
        description="این صفحه بارگذاری شده ولی درخواست‌های API به سرور نمی‌رسند. معمولاً یکی از این‌هاست: فیلترشکن/پروکسی سیستم (v2rayN و…) که ترافیک localhost را هم رد می‌کند — آن را روی «Bypass» برای آدرس‌های محلی بگذارید یا موقتاً خاموش کنید؛ یا افزونهٔ ad-blocker روی این صفحه؛ یا اینکه پنل را روی پورت اشتباه باز کرده‌اید (باید همان پورتی باشد که کانتینر منتشر کرده). بعد Ctrl+Shift+R."
      />

      <a-alert
        v-if="store.sessionExpiredNote"
        type="warning"
        :message="store.sessionExpiredNote"
        show-icon
        banner
        style="margin-bottom: 14px"
      />
      <a-alert v-if="error" type="error" :message="error" show-icon banner style="margin-bottom: 14px" />

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

      <ul class="login-features">
        <li><AppIcon name="file-code" :size="14" /> طراحی چالش، سطح دشواری و مثال‌ها</li>
        <li><AppIcon name="activity" :size="14" /> نظارت زندهٔ وضعیت شرکت‌کنندگان</li>
        <li><AppIcon name="wand" :size="14" /> بررسی هوشمند احتمال تولید کد با AI</li>
      </ul>

      <a class="login-back" href="/">
        <AppIcon name="chevron-right" :size="14" />
        بازگشت به سامانهٔ چالش
      </a>
    </div>
  </div>
</template>

<style scoped>
.login-shell {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: var(--space-5);
  position: relative;
}
.login-top {
  position: absolute;
  top: var(--space-5);
  inset-inline-end: var(--space-5);
}
.login-card {
  width: 100%;
  max-width: 420px;
  padding: var(--space-6);
}
.login-head {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-5);
}
.login-head h1 {
  font-size: 1.1rem;
  margin: 0;
}
.brand-mark {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  background: var(--primary);
  color: var(--on-primary);
  flex: none;
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
.login-features {
  list-style: none;
  margin: var(--space-4) 0 0;
  padding: var(--space-4) 0 0;
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.login-features li {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.76rem;
  color: var(--text-dim);
}
.login-features li :deep(.app-icon) {
  color: var(--primary-bright);
}
.login-back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: var(--space-4);
  font-size: 0.78rem;
  color: var(--text-faint);
}
.login-back:hover {
  color: var(--text);
}
</style>
