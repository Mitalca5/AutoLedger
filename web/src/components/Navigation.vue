<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useQuickAddStore } from '@/stores/quickAdd'
import { useVehicleStore } from '@/stores/vehicle'
import {
  LayoutDashboard,
  Navigation as NavIcon,
  Users,
  Disc,
  Receipt,
  Car,
  LogOut,
  Zap,
  Scale,
  ClipboardList,
  UserRound,
  Plus,
  Ellipsis,
  X,
  Gauge,
} from 'lucide-vue-next'
import { APP_NAME } from '@/brand'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import { APP_VERSION } from '@/version'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const vehicleStore = useVehicleStore()
const quickAdd = useQuickAddStore()

const allNavItems = [
  { name: 'dashboard', labelKey: 'shell.nav.dashboard', mobileLabelKey: 'shell.nav.home', path: '/', icon: LayoutDashboard },
  { name: 'fleet', labelKey: 'shell.nav.fleet', path: '/fleet', icon: Gauge },
  { name: 'drives', labelKey: 'shell.nav.drives', path: '/drives', icon: NavIcon },
  { name: 'carpools', labelKey: 'shell.nav.carpools', path: '/carpools', icon: Users },
  { name: 'tires', labelKey: 'shell.nav.tires', path: '/tires', icon: Disc },
  { name: 'manual', labelKey: 'shell.nav.manual', path: '/manual', icon: ClipboardList },
  { name: 'expenses', labelKey: 'shell.nav.expenses', path: '/expenses', icon: Receipt },
  { name: 'comparison', labelKey: 'shell.nav.comparison', path: '/comparison', icon: Scale },
  { name: 'vehicles', labelKey: 'shell.nav.vehicles', path: '/vehicles', icon: Car },
  { name: 'account', labelKey: 'shell.nav.account', path: '/account', icon: UserRound },
]

// Drives and carpooling apply to any non-combustion vehicle (connected or manually tracked)
const navItems = computed(() =>
  allNavItems.filter((item) => {
    if (item.name === 'drives') return vehicleStore.canCharge
    if (item.name === 'carpools') return vehicleStore.canCharge
    return true
  }),
)

const currentRouteName = computed(() => route.name)

// Phone bar: the everyday pages, the quick entry button in the middle, everything else behind "Plus".
const primaryNames = computed(() => (vehicleStore.canCharge ? ['dashboard', 'drives', 'expenses'] : ['dashboard', 'expenses', 'manual']))
const primaryItems = computed(() =>
  primaryNames.value.map((name) => navItems.value.find((item) => item.name === name)).filter((item) => !!item),
)
const secondaryItems = computed(() => navItems.value.filter((item) => !primaryNames.value.includes(item.name)))
const leftItems = computed(() => primaryItems.value.slice(0, 2))
const rightItems = computed(() => primaryItems.value.slice(2))
const moreActive = computed(() => secondaryItems.value.some((item) => item.name === currentRouteName.value))

const canQuickAdd = computed(() => !!vehicleStore.activeVehicle && vehicleStore.canEdit)
const showMore = ref(false)

watch(() => route.path, () => {
  showMore.value = false
})

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') showMore.value = false
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))

function handleLogout() {
  showMore.value = false
  authStore.logout()
  router.push('/login')
}
</script>

<template>
  <!-- Desktop Sidebar -->
  <aside class="hidden md:flex flex-col w-64 bg-slate-900 border-r border-slate-800 p-4 shrink-0">
    <div class="flex items-center gap-3 px-3 py-4 mb-4">
      <div class="p-2 bg-gradient-to-tr from-rose-500 to-warning-500 rounded-xl shadow-lg shadow-rose-500/20">
        <Zap class="w-6 h-6 text-white" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <p class="text-xl font-bold tracking-tight bg-gradient-to-r from-white to-slate-400 bg-clip-text text-transparent">
            {{ APP_NAME }}
          </p>
        </div>
        <p class="text-xs text-slate-400">TCO & Fleet Manager</p>
      </div>
    </div>

    <button
      v-if="canQuickAdd"
      type="button"
      class="mb-3 flex items-center justify-center gap-2 rounded-xl bg-rose-600 px-3 py-2.5 text-sm font-bold text-white shadow-lg shadow-rose-600/20 transition-colors hover:bg-rose-500"
      @click="quickAdd.open()"
    >
      <Plus class="h-4 w-4" aria-hidden="true" />
      {{ $t('shell.navigation.quickAdd') }}
    </button>

    <nav class="flex-1 space-y-1">
      <router-link
        v-for="item in navItems"
        :key="item.name"
        :to="item.path"
        class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-all border border-transparent"
        :class="
          currentRouteName === item.name
            ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20 shadow-sm'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
        "
      >
        <component :is="item.icon" class="w-5 h-5" />
        {{ $t(item.labelKey) }}
      </router-link>
    </nav>

    <div class="pt-4 border-t border-slate-800 mt-auto space-y-2">
      <div class="flex items-center justify-between px-3 py-1">
        <div class="truncate">
          <p class="text-xs font-semibold text-slate-200 truncate">{{ authStore.user?.email }}</p>
          <p class="text-xs text-slate-400">{{ $t('shell.navigation.signedIn') }}</p>
        </div>
        <button
          @click="handleLogout"
          class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
          :title="$t('shell.navigation.signOut')"
        >
          <LogOut class="w-4 h-4" />
        </button>
      </div>
      <div class="px-3 pt-2 text-xs text-slate-400 flex items-center justify-between border-t border-slate-800/60">
        <span>{{ $t('shell.navigation.version') }}</span>
        <span class="font-mono text-slate-400 font-medium">{{ APP_VERSION }}</span>
      </div>
      <div class="px-3 pt-2">
        <LanguageSwitcher />
      </div>
    </div>
  </aside>

  <!-- Mobile Bottom Navigation Bar -->
  <nav :aria-label="$t('shell.navigation.mainNavigation')" class="md:hidden fixed bottom-0 left-0 right-0 z-50 bg-slate-900/95 backdrop-blur-md border-t border-slate-800 px-1 safe-area-pb">
    <ul class="flex items-stretch">
      <li v-for="item in leftItems" :key="item.name" class="flex-1">
        <router-link
          :to="item.path"
          class="flex min-h-14 flex-col items-center justify-center gap-0.5 rounded-lg text-xs font-medium transition-colors"
          :class="currentRouteName === item.name ? 'text-rose-400 font-semibold' : 'text-slate-400 hover:text-slate-200'"
          :aria-current="currentRouteName === item.name ? 'page' : undefined"
        >
          <component :is="item.icon" class="h-5 w-5" aria-hidden="true" />
          <span>{{ $t(item.mobileLabelKey ?? item.labelKey) }}</span>
        </router-link>
      </li>

      <li class="flex flex-1 items-center justify-center">
        <button
          v-if="canQuickAdd"
          type="button"
          class="-mt-5 flex h-14 w-14 items-center justify-center rounded-full bg-rose-600 text-white shadow-lg shadow-rose-600/40 ring-4 ring-slate-950 transition-colors hover:bg-rose-500 active:bg-rose-700"
          :aria-label="$t('shell.navigation.quickAdd')"
          @click="quickAdd.open()"
        >
          <Plus class="h-7 w-7" aria-hidden="true" />
        </button>
      </li>

      <li v-for="item in rightItems" :key="item.name" class="flex-1">
        <router-link
          :to="item.path"
          class="flex min-h-14 flex-col items-center justify-center gap-0.5 rounded-lg text-xs font-medium transition-colors"
          :class="currentRouteName === item.name ? 'text-rose-400 font-semibold' : 'text-slate-400 hover:text-slate-200'"
          :aria-current="currentRouteName === item.name ? 'page' : undefined"
        >
          <component :is="item.icon" class="h-5 w-5" aria-hidden="true" />
          <span>{{ $t(item.mobileLabelKey ?? item.labelKey) }}</span>
        </router-link>
      </li>

      <li class="flex-1">
        <button
          type="button"
          class="flex min-h-14 w-full flex-col items-center justify-center gap-0.5 rounded-lg text-xs font-medium transition-colors"
          :class="moreActive || showMore ? 'text-rose-400 font-semibold' : 'text-slate-400 hover:text-slate-200'"
          aria-haspopup="dialog"
          :aria-expanded="showMore"
          @click="showMore = true"
        >
          <Ellipsis class="h-5 w-5" aria-hidden="true" />
          <span>{{ $t('shell.navigation.more') }}</span>
        </button>
      </li>
    </ul>
  </nav>

  <!-- Mobile "Plus" sheet -->
  <div
    v-if="showMore"
    class="md:hidden fixed inset-0 z-[60] flex items-end bg-black/70"
    @click.self="showMore = false"
  >
    <div
      v-dialog="() => (showMore = false)"
      role="dialog"
      aria-modal="true"
      :aria-label="$t('shell.navigation.otherPages')"
      class="w-full rounded-t-3xl border border-slate-800 bg-slate-900 px-4 pt-4 pb-[max(1rem,env(safe-area-inset-bottom))] shadow-2xl"
    >
      <div class="mb-2 flex items-center justify-between">
        <p class="truncate text-xs text-slate-400">{{ authStore.user?.email }}</p>
        <button type="button" class="flex h-11 w-11 items-center justify-center rounded-xl text-slate-400 hover:bg-slate-800 hover:text-white" :aria-label="$t('common.close')" @click="showMore = false">
          <X class="h-5 w-5" aria-hidden="true" />
        </button>
      </div>
      <ul class="space-y-1">
        <li v-for="item in secondaryItems" :key="item.name">
          <router-link
            :to="item.path"
            class="flex min-h-12 items-center gap-3 rounded-xl px-3 text-base font-medium"
            :class="currentRouteName === item.name ? 'bg-rose-500/10 text-rose-300' : 'text-slate-200 hover:bg-slate-800'"
            :aria-current="currentRouteName === item.name ? 'page' : undefined"
          >
            <component :is="item.icon" class="h-5 w-5" aria-hidden="true" />
            {{ $t(item.labelKey) }}
          </router-link>
        </li>
        <li>
          <button type="button" class="flex min-h-12 w-full items-center gap-3 rounded-xl px-3 text-base font-medium text-slate-300 hover:bg-slate-800" @click="handleLogout">
            <LogOut class="h-5 w-5" aria-hidden="true" />
            {{ $t('shell.navigation.signOut') }}
          </button>
        </li>
      </ul>
      <p class="mt-3 text-center font-mono text-xs text-slate-400">{{ APP_NAME }} {{ APP_VERSION }}</p>
    </div>
  </div>
</template>
