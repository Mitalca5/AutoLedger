<script setup lang="ts">
import { onMounted, onUnmounted, computed, watch, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { APP_NAME } from '@/brand'
import { useAuthStore } from '@/stores/auth'
import { useVehicleStore } from '@/stores/vehicle'
import { useOfflineStore } from '@/stores/offline'
import Navigation from '@/components/Navigation.vue'
import TopBar from '@/components/TopBar.vue'
import ConfirmModal from '@/components/ConfirmModal.vue'
import ToastHost from '@/components/ToastHost.vue'
import QuickAddSheet from '@/components/quickadd/QuickAddSheet.vue'

const route = useRoute()
const { t } = useI18n()

const ROUTE_TITLE_KEYS: Record<string, string> = {
  dashboard: 'shell.nav.dashboard',
  fleet: 'shell.nav.fleet',
  drives: 'shell.nav.drives',
  carpools: 'shell.nav.carpools',
  tires: 'shell.nav.tires',
  odometer: 'shell.nav.odometer',
  expenses: 'shell.nav.expenses',
  comparison: 'shell.nav.comparison',
  vehicles: 'shell.nav.vehicles',
  account: 'shell.nav.account',
  login: 'auth.loginView.signInTo',
  register: 'auth.registerView.createAnAccount',
  onboarding: 'onboarding.onboardingView.welcomeTo',
}

const pageTitle = computed(() => {
  const key = ROUTE_TITLE_KEYS[String(route.name)]
  return key ? t(key, { APP_NAME }) : APP_NAME
})

watch(
  pageTitle,
  (title) => {
    document.title = title === APP_NAME ? APP_NAME : `${title} · ${APP_NAME}`
  },
  { immediate: true }
)
const authStore = useAuthStore()
const vehicleStore = useVehicleStore()

const showDashboardLayout = computed(() => {
  return (
    authStore.isAuthenticated &&
    route.name !== 'onboarding' &&
    route.name !== 'login' &&
    route.name !== 'register'
  )
})

const offlineStore = useOfflineStore()
const isDemo = ref(false)

// Pages follow the server-side synchronization while the user is signed in
watch(
  () => authStore.isAuthenticated,
  (authenticated) => (authenticated ? vehicleStore.startAutoRefresh() : vehicleStore.stopAutoRefresh()),
  { immediate: true }
)
onUnmounted(() => vehicleStore.stopAutoRefresh())

onMounted(async () => {
  offlineStore.start()
  fetch('/api/auth/config')
    .then((res) => (res.ok ? res.json() : null))
    .then((data) => (isDemo.value = !!data?.demo))
    .catch(() => {})
  await authStore.init()
  if (authStore.isAuthenticated) {
    await vehicleStore.fetchVehicles()
    vehicleStore.resumeRunningSync()
  }
})
</script>

<template>
  <div v-if="showDashboardLayout" class="flex h-screen overflow-hidden bg-slate-950">
    <a href="#main-content" class="sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-[100] focus:rounded-lg focus:bg-rose-600 focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-white">{{ $t('shell.a11y.skipToContent') }}</a>
    <Navigation />
    <div class="flex-1 flex flex-col min-w-0 overflow-y-auto overflow-x-hidden pb-[calc(5.5rem+env(safe-area-inset-bottom))] md:pb-0">
      <TopBar />
      <div v-if="isDemo" role="status" class="bg-warning-500/10 border-b border-warning-500/20 px-4 py-2 text-center text-xs font-medium text-warning-300">
        {{ $t('shell.app.demoBanner') }}
      </div>
      <main id="main-content" tabindex="-1" class="flex-1 p-4 md:p-6 max-w-7xl w-full mx-auto focus:outline-none">
        <h1 class="sr-only">{{ pageTitle }}</h1>
        <!-- Attente de l'initialisation du store véhicule pour éviter un affichage vide au refresh -->
        <div v-if="!vehicleStore.isInitialized" class="flex flex-col items-center justify-center py-28 space-y-4">
          <div class="w-9 h-9 border-3 border-rose-500 border-t-transparent rounded-full animate-spin"></div>
          <p class="text-xs font-medium text-slate-400">{{ $t('shell.app.loadingYourVehicle') }}</p>
        </div>
        <router-view v-else />
      </main>
    </div>
  </div>

  <div v-else class="h-full overflow-y-auto bg-slate-950">
    <router-view />
  </div>

  <QuickAddSheet v-if="showDashboardLayout" />
  <ConfirmModal />
  <ToastHost />
</template>
