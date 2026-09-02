<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { ApiError } from '@/api/client'
import AppIcon from '@/components/AppIcon.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

const store = useAdminStore()
const form = reactive({ username: 'administrator', password: '' })
const showPass = ref(false)
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const ok = await store.login(form.username.trim(), form.password)
    if (!ok) error.value = 'نام کاربری یا رمز عبور اشتباه است.'
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'ورود ناموفق بود'
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
</style>
