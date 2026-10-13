<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Modal } from 'ant-design-vue'
import { useAdminStore } from '@/stores/admin'
import { getVersion } from '@/api/client'
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
interface NavGroup {
  label: string
  items: NavItem[]
}
const NAV: NavGroup[] = [
  {
    label: 'بازی',
    items: [
      { name: 'admin-game', label: 'نتایج زنده بازی', icon: 'trophy' },
      { name: 'admin-game-top', label: '۳ نفر برتر', icon: 'crown' },
      { name: 'admin-game-questions', label: 'بانک سوالات بازی', icon: 'help-circle' },
    ],
  },
  {
    label: 'چالش برنامه‌نویسی',
    items: [
      { name: 'admin-overview', label: 'نمای کلی رویداد', icon: 'activity' },
      { name: 'admin-schedule', label: 'زمان‌بندی', icon: 'calendar' },
      { name: 'admin-challenge', label: 'طراحی چالش', icon: 'file-code' },
    ],
  },
  {
    label: 'کاربران',
    items: [
      { name: 'admin-participants', label: 'شرکت‌کنندگان', icon: 'users' },
      { name: 'admin-errors', label: 'خطاهای کاربران', icon: 'alert-triangle', badge: true },
    ],
  },
  {
    label: 'هوش مصنوعی',
    items: [
      { name: 'admin-ai', label: 'بررسی هوشمند', icon: 'wand' },
      { name: 'admin-assistant', label: 'دستیار کد', icon: 'code' },
    ],
  },
  {
    label: 'سامانه',
    items: [
      { name: 'admin-appearance', label: 'ظاهر و طراحی', icon: 'palette' },
      { name: 'admin-resources', label: 'منابع سرور', icon: 'cpu' },
    ],
  },
]
const ALL_ITEMS = NAV.flatMap((g) => g.items)
const currentLabel = computed(() => ALL_ITEMS.find((n) => n.name === route.name)?.label || '')

/* Collapsible desktop rail — icon-only at 64px, remembered per browser. */
const RAIL_KEY = 'dotinschool_admin_rail'
const railed = ref(false)
try {
  railed.value = localStorage.getItem(RAIL_KEY) === '1'
} catch {
  /* private mode */
}
function toggleRail() {
  railed.value = !railed.value
  try {
    localStorage.setItem(RAIL_KEY, railed.value ? '1' : '0')
  } catch {
    /* ignore */
  }
}

const appVersion = ref('')
onMounted(() => {
  if (store.authed) store.startPolling()
  getVersion()
    .then((r) => (appVersion.value = r.version || ''))
    .catch(() => {})
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

  <div v-else class="admin-shell" :class="{ railed }">
    <aside class="sidebar" :class="{ open: drawerOpen }">
      <div class="side-brand">
        <span class="brand-mark"><AppIcon name="shield" :size="18" /></span>
        <span class="brand-name">dotinschool</span>
        <button
          type="button"
          class="rail-toggle only-desktop"
          :aria-label="railed ? 'باز کردن نوار کناری' : 'جمع‌کردن نوار کناری'"
          @click="toggleRail"
        >
          <AppIcon :name="railed ? 'chevron-left' : 'chevron-right'" :size="16" />
        </button>
      </div>

      <nav class="side-nav">
        <div v-for="group in NAV" :key="group.label" class="nav-group">
          <span class="nav-group-label">{{ group.label }}</span>
          <router-link
            v-for="item in group.items"
            :key="item.name"
            :to="{ name: item.name }"
            class="nav-item"
            active-class="active"
            :title="railed ? item.label : undefined"
            @click="drawerOpen = false"
          >
            <span class="nav-ico"><AppIcon :name="item.icon" :size="18" /></span>
            <span class="nav-label">{{ item.label }}</span>
            <span v-if="item.badge && store.unseenErrors > 0" class="nav-badge num">
              {{ toFa(store.unseenErrors) }}
            </span>
          </router-link>
        </div>
      </nav>

      <div class="side-foot faint">نسخه {{ appVersion ? toFa(appVersion) : '—' }}</div>
    </aside>

    <div v-if="drawerOpen" class="scrim" @click="drawerOpen = false" />

    <div class="admin-main">
      <header class="topbar">
        <button
          type="button"
          class="icon-btn only-mobile"
          aria-label="منو"
          @click="drawerOpen = true"
        >
          <AppIcon name="menu" :size="20" />
        </button>
        <button
          type="button"
          class="icon-btn only-desktop"
          :aria-label="railed ? 'باز کردن نوار کناری' : 'جمع‌کردن نوار کناری'"
          @click="toggleRail"
        >
          <AppIcon name="menu" :size="18" />
        </button>

        <nav class="crumbs" aria-label="مسیر">
          <span class="crumb-root">پنل مدیریت</span>
          <AppIcon name="chevron-left" :size="14" class="crumb-sep" />
          <span class="crumb-cur">{{ currentLabel }}</span>
        </nav>

        <div class="grow" />

        <span class="live" title="داده‌ها هر چند ثانیه به‌روزرسانی می‌شوند">
          <i class="live-dot" />زنده
        </span>
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

/* ---------- sidebar ---------- */
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
  transition: width var(--dur) var(--ease);
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
  flex: none;
  border-radius: var(--radius-sm);
  background: var(--primary);
  color: var(--on-primary);
}
.brand-name {
  font-weight: 800;
  font-size: 1rem;
  letter-spacing: 0.01em;
  white-space: nowrap;
  overflow: hidden;
}
.rail-toggle {
  margin-inline-start: auto;
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border-radius: 7px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text-faint);
  transition: color var(--dur) var(--ease), background var(--dur) var(--ease);
}
.rail-toggle:hover {
  color: var(--text);
  background: var(--surface-2);
}

.side-nav {
  flex: 1;
  padding: var(--space-3) var(--space-2);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  overflow-y: auto;
  overflow-x: hidden;
}
.nav-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.nav-group-label {
  padding: 0 12px 4px;
  font-size: 0.68rem;
  font-weight: 700;
  color: var(--text-faint);
  white-space: nowrap;
}
.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  color: var(--text-dim);
  font-size: 0.86rem;
  font-weight: 500;
  white-space: nowrap;
  transition: color var(--dur) var(--ease), background var(--dur) var(--ease);
}
.nav-ico {
  display: grid;
  place-items: center;
  width: 20px;
  flex: none;
}
.nav-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
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
/* edge accent bar on the active item (inside edge in RTL) */
.nav-item.active::before {
  content: '';
  position: absolute;
  inset-inline-start: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 20px;
  border-radius: 0 var(--radius-pill) var(--radius-pill) 0;
  background: var(--primary);
}
[dir='rtl'] .nav-item.active::before {
  border-radius: var(--radius-pill) 0 0 var(--radius-pill);
}
.nav-badge {
  margin-inline-start: auto;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  display: grid;
  place-items: center;
  border-radius: var(--radius-pill);
  background: var(--danger);
  color: #fff;
  font-size: 0.66rem;
  font-weight: 700;
}
.side-foot {
  padding: var(--space-3) var(--space-4);
  font-size: 0.7rem;
  border-top: 1px solid var(--border);
  white-space: nowrap;
  overflow: hidden;
}

/* ---------- railed (collapsed) state, desktop only ---------- */
@media (min-width: 901px) {
  .admin-shell.railed .sidebar {
    width: var(--rail-w);
  }
  .admin-shell.railed .brand-name,
  .admin-shell.railed .nav-group-label,
  .admin-shell.railed .nav-label,
  .admin-shell.railed .nav-badge,
  .admin-shell.railed .side-foot {
    display: none;
  }
  .admin-shell.railed .side-brand {
    padding: 0;
    justify-content: center;
  }
  .admin-shell.railed .rail-toggle {
    display: none;
  }
  .admin-shell.railed .side-nav {
    align-items: center;
  }
  .admin-shell.railed .nav-group {
    width: 100%;
    align-items: center;
    padding-top: var(--space-2);
    border-top: 1px solid var(--border);
  }
  .admin-shell.railed .nav-group:first-child {
    border-top: 0;
    padding-top: 0;
  }
  .admin-shell.railed .nav-item {
    width: 40px;
    padding: 10px;
    justify-content: center;
  }
}

/* ---------- main column ---------- */
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
  background: color-mix(in srgb, var(--bg) 82%, transparent);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--border);
}
.crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.crumb-root {
  font-size: 0.8rem;
  color: var(--text-faint);
  white-space: nowrap;
}
.crumb-sep {
  color: var(--text-faint);
  flex: none;
}
.crumb-cur {
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-dim);
  padding: 3px 9px;
  border: 1px solid var(--border);
  border-radius: var(--radius-pill);
}
.live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--success);
  box-shadow: 0 0 0 0 color-mix(in srgb, var(--success) 60%, transparent);
  animation: live-pulse 2s var(--ease) infinite;
}
@keyframes live-pulse {
  0% {
    box-shadow: 0 0 0 0 color-mix(in srgb, var(--success) 55%, transparent);
  }
  70% {
    box-shadow: 0 0 0 6px transparent;
  }
  100% {
    box-shadow: 0 0 0 0 transparent;
  }
}
@media (prefers-reduced-motion: reduce) {
  .live-dot {
    animation: none;
  }
}
.who {
  font-size: 0.78rem;
}
.icon-btn {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  flex: none;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text-dim);
  transition: color var(--dur) var(--ease), background var(--dur) var(--ease);
}
.icon-btn:hover {
  color: var(--text);
  background: var(--surface-2);
}
.content {
  flex: 1;
  padding: var(--space-5);
  max-width: var(--content-max, 1280px);
  width: 100%;
  margin: 0 auto;
}

.only-mobile {
  display: none;
}
.only-desktop {
  display: grid;
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
    width: var(--sidebar-w);
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
  .only-desktop {
    display: none;
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
