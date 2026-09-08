<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Modal } from 'ant-design-vue'
import { useAdminStore } from '@/stores/admin'
import { toFa } from '@/lib/fa'
import AdminLogin from './AdminLogin.vue'
import AppIcon from '@/components/AppIcon.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

const store = useAdminStore()
const route = useRoute()
const drawerOpen = ref(false)

interface NavItem {
  name: string
  label: string
  icon: string
  badge?: boolean
}
const NAV: NavItem[] = [
  { name: 'admin-overview', label: 'نمای کلی رویداد', icon: 'activity' },
  { name: 'admin-schedule', label: 'زمان‌بندی', icon: 'calendar' },
  { name: 'admin-challenge', label: 'طراحی چالش', icon: 'file-code' },
  { name: 'admin-participants', label: 'شرکت‌کنندگان', icon: 'users' },
  { name: 'admin-ai', label: 'بررسی هوشمند', icon: 'wand' },
  { name: 'admin-assistant', label: 'دستیار کد', icon: 'code' },
  { name: 'admin-appearance', label: 'ظاهر و طراحی', icon: 'palette' },
  { name: 'admin-errors', label: 'خطاهای کاربران', icon: 'alert-triangle', badge: true },
]

const currentLabel = computed(() => NAV.find((n) => n.name === route.name)?.label || '')

onMounted(() => {
  if (store.authed) store.startPolling()
})
onBeforeUnmount(() => store.stopPolling())

watch(
  () => store.authed,
  (v) => (v ? store.startPolling() : store.stopPolling()),
)

function confirmLogout() {
  Modal.confirm({
    title: 'خروج از پنل مدیریت',
    content: 'برای ورود دوباره باید نام کاربری و رمز عبور را وارد کنید.',
    okText: 'خروج',
    cancelText: 'انصراف',
    okType: 'danger',
    onOk: () => store.logout(),
  })
}
</script>

<template>
  <AdminLogin v-if="!store.authed" />

  <div v-else class="admin-shell">
    <aside class="sidebar" :class="{ open: drawerOpen }">
      <div class="side-brand">
        <span class="brand-mark"><AppIcon name="shield" :size="18" /></span>
        <span class="brand-name">dotinschool</span>
      </div>
      <nav class="side-nav">
        <router-link
          v-for="item in NAV"
          :key="item.name"
          :to="{ name: item.name }"
          class="nav-item"
          active-class="active"
          @click="drawerOpen = false"
        >
          <AppIcon :name="item.icon" :size="18" />
          <span>{{ item.label }}</span>
          <span v-if="item.badge && store.unseenErrors > 0" class="nav-badge num">
            {{ toFa(store.unseenErrors) }}
          </span>
        </router-link>
      </nav>
      <div class="side-foot faint">نسخه {{ toFa('1.0') }}</div>
    </aside>

    <div v-if="drawerOpen" class="scrim" @click="drawerOpen = false" />

    <div class="admin-main">
      <header class="topbar">
        <button type="button" class="icon-btn only-mobile" aria-label="منو" @click="drawerOpen = true">
          <AppIcon name="menu" :size="20" />
        </button>
        <h1 class="topbar-title">{{ currentLabel }}</h1>
        <div class="grow" />
        <span class="who faint">{{ store.username }}</span>
        <ThemeToggle />
        <button type="button" class="icon-btn" title="خروج" @click="confirmLogout">
          <AppIcon name="log-out" :size="18" />
        </button>
      </header>

      <main class="content">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin-shell {
  min-height: 100dvh;
  display: flex;
}
.sidebar {
  width: var(--sidebar-w);
  flex: none;
  background: var(--surface);
  border-inline-end: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 0;
  height: 100dvh;
}
.side-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 var(--space-4);
  height: var(--header-h);
  border-bottom: 1px solid var(--border);
}
.brand-mark {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  background: var(--primary);
  color: var(--on-primary);
}
.brand-name {
  font-weight: 800;
  font-size: 1rem;
}
.side-nav {
  flex: 1;
  padding: var(--space-3);
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow-y: auto;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  color: var(--text-dim);
  font-size: 0.86rem;
  font-weight: 500;
  transition: all var(--dur) var(--ease);
}
.nav-item:hover {
  background: var(--surface-2);
  color: var(--text);
}
.nav-item.active {
  background: var(--primary-soft);
  color: var(--primary-bright);
  font-weight: 600;
}
.nav-badge {
  margin-inline-start: auto;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  display: grid;
  place-items: center;
  border-radius: 999px;
  background: var(--danger);
  color: #fff;
  font-size: 0.66rem;
  font-weight: 700;
}
.side-foot {
  padding: var(--space-3) var(--space-4);
  font-size: 0.7rem;
  border-top: 1px solid var(--border);
}
.admin-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.topbar {
  position: sticky;
  top: 0;
  z-index: 40;
  display: flex;
  align-items: center;
  gap: var(--space-3);
  height: var(--header-h);
  padding: 0 var(--space-5);
  background: color-mix(in srgb, var(--bg) 85%, transparent);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border);
}
.topbar-title {
  font-size: 1rem;
  margin: 0;
}
.who {
  font-size: 0.78rem;
}
.icon-btn {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text-dim);
}
.icon-btn:hover {
  color: var(--text);
  background: var(--surface-2);
}
.content {
  flex: 1;
  padding: var(--space-5);
  max-width: 1280px;
  width: 100%;
  margin: 0 auto;
}
.only-mobile {
  display: none;
}
.scrim {
  display: none;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s var(--ease);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@media (max-width: 900px) {
  .sidebar {
    position: fixed;
    inset-block: 0;
    inset-inline-start: 0;
    z-index: 60;
    transform: translateX(100%);
    transition: transform var(--dur) var(--ease);
  }
  [dir='rtl'] .sidebar {
    transform: translateX(100%);
  }
  .sidebar.open {
    transform: translateX(0);
  }
  .only-mobile {
    display: grid;
  }
  .scrim {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 55;
    background: rgba(2, 6, 23, 0.5);
  }
}
</style>
