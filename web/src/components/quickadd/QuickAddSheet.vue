<script setup lang="ts">
import { t } from '@/i18n'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CheckCircle2, X } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { useOfflineStore } from '@/stores/offline'
import { useQuickAddStore } from '@/stores/quickAdd'
import { useVehicleStore } from '@/stores/vehicle'
import type { PendingCharge, QuickKind } from '@/utils/quickAdd'
import QuickChargeForm from './QuickChargeForm.vue'
import QuickExpenseForm from './QuickExpenseForm.vue'
import QuickFuelForm from './QuickFuelForm.vue'
import QuickPendingCosts from './QuickPendingCosts.vue'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const vehicleStore = useVehicleStore()
const offlineStore = useOfflineStore()
const quickAdd = useQuickAddStore()

const PENDING_LIMIT = 5

const pending = ref<PendingCharge[]>([])
const pendingTotal = ref(0)

const vehicle = computed(() => vehicleStore.activeVehicle)

const tabs = computed<{ key: QuickKind; label: string; badge?: number }[]>(() => {
  const list: { key: QuickKind; label: string; badge?: number }[] = []
  if (vehicleStore.canCharge) {
    if (pendingTotal.value > 0) list.push({ key: 'PENDING', label: 'quickadd.quickAddSheet.tabPending', badge: pendingTotal.value })
    list.push({ key: 'CHARGE', label: 'quickadd.quickAddSheet.tabCharge' })
  }
  if (vehicleStore.canRefuel) list.push({ key: 'FUEL', label: 'quickadd.quickAddSheet.tabFuel' })
  list.push({ key: 'EXPENSE', label: 'quickadd.quickAddSheet.tabExpense' })
  return list
})

const activeKind = ref<QuickKind>('CHARGE')

// Recharges TeslaMate waiting for a cost: loaded ahead of time so the sheet opens on the right tab.
async function loadPending() {
  if (!vehicle.value || !vehicleStore.canCharge || !vehicleStore.canEdit || !offlineStore.isOnline) return
  try {
    const res = await api.getCharges(vehicle.value.id, { missingCost: true, limit: PENDING_LIMIT })
    pending.value = (res.charges || []).filter((c: any) => !c.is_manual)
    pendingTotal.value = res.charges_without_cost || 0
  } catch {
    // Offline or transient error: keep what is known, the sheet stays usable without this list
  }
}

watch(() => vehicle.value?.id, () => {
  pending.value = []
  pendingTotal.value = 0
  loadPending()
}, { immediate: true })

// A synchronization or a saved entry may have changed which charges lack a cost.
watch(() => vehicleStore.lastSyncTimestamp, loadPending)

// Where focus goes back to when the sheet closes.
let previouslyFocused: HTMLElement | null = null
const panel = ref<HTMLElement | null>(null)

watch(() => quickAdd.isOpen, async (open) => {
  if (open) {
    previouslyFocused = document.activeElement as HTMLElement | null
    const wanted = quickAdd.requestedKind
    const available = tabs.value.map((t) => t.key)
    const fallback: QuickKind = available.includes('PENDING') ? 'PENDING' : available[0]
    activeKind.value = wanted && available.includes(wanted) ? wanted : fallback
    loadPending()
    await nextTick()
    trackViewport()
  } else {
    untrackViewport()
    previouslyFocused?.focus?.()
    previouslyFocused = null
  }
})

// A vehicle without edit rights, or none at all, has nothing to enter.
watch(() => vehicleStore.canEdit && !!vehicle.value, (ok) => {
  if (!ok && quickAdd.isOpen) quickAdd.close()
})

// Only a page change closes it: dropping ?quick= from the URL must not.
watch(() => route.path, () => {
  if (quickAdd.isOpen) quickAdd.close()
})

// PWA shortcuts open the sheet through ?quick=<kind>; the parameter is then dropped from the URL.
function kindFromQuery(value: string): QuickKind | null {
  if (value === 'expense') return 'EXPENSE'
  if (value === 'charge') return 'CHARGE'
  if (value === 'fuel') return 'FUEL'
  return null
}

watch(
  () => [route.query.quick, vehicleStore.isInitialized, authStore.isAuthenticated] as const,
  ([quick, ready, authed]) => {
    if (typeof quick !== 'string' || !ready || !authed) return
    const rest = { ...route.query }
    delete rest.quick
    router.replace({ query: rest })
    if (vehicle.value && vehicleStore.canEdit) quickAdd.open(kindFromQuery(quick))
  },
  { immediate: true },
)

// Confirmation shown once the sheet is closed. An entry stored for later is announced by the offline banner of
// the top bar, so only saved entries get a toast.
const toast = ref<string | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | undefined

function showToast(message: string) {
  toast.value = message
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toast.value = null), 3500)
}

function onSaved(result: { queued: boolean; message: string }) {
  quickAdd.close()
  if (result.queued) return
  showToast(t('quickadd.quickAddSheet.saved', { message: result.message }))
  // Refreshes the odometer pill and the pages listing the new entry
  vehicleStore.fetchVehicles()
}

function onCompleted(result: { id: string; queued: boolean; message: string }) {
  pending.value = pending.value.filter((c) => c.id !== result.id)
  pendingTotal.value = Math.max(0, pendingTotal.value - 1)
  if (!result.queued) {
    showToast(result.message)
    vehicleStore.fetchVehicles()
  }
  // Nothing left to complete: the task is done
  if (pendingTotal.value === 0) quickAdd.close()
}

// The on-screen keyboard does not shrink the layout viewport on iOS: the sheet is lifted by the covered height.
const keyboardOffset = ref(0)
const visibleHeight = ref<number | null>(null)

function syncViewport() {
  const vv = window.visualViewport
  if (!vv) return
  keyboardOffset.value = Math.max(0, window.innerHeight - vv.height - vv.offsetTop)
  visibleHeight.value = vv.height
}

function trackViewport() {
  window.visualViewport?.addEventListener('resize', syncViewport)
  window.visualViewport?.addEventListener('scroll', syncViewport)
  syncViewport()
}

function untrackViewport() {
  window.visualViewport?.removeEventListener('resize', syncViewport)
  window.visualViewport?.removeEventListener('scroll', syncViewport)
  keyboardOffset.value = 0
  visibleHeight.value = null
}

const panelStyle = computed(() => ({
  bottom: `${keyboardOffset.value}px`,
  maxHeight: visibleHeight.value ? `${Math.floor(visibleHeight.value * 0.94)}px` : '92dvh',
}))

const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

function onKeydown(event: KeyboardEvent) {
  if (!quickAdd.isOpen) return
  if (event.key !== 'Tab' || !panel.value) return
  const items = Array.from(panel.value.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((el) => el.offsetParent !== null)
  if (items.length === 0) return
  const first = items[0]
  const last = items[items.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

useEscapeToClose(() => quickAdd.isOpen, () => quickAdd.close())

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  untrackViewport()
  clearTimeout(toastTimer)
})
</script>

<template>
  <div
    v-if="quickAdd.isOpen && vehicle"
    class="fixed inset-0 z-[70] flex items-end justify-center bg-black/70 md:items-center md:p-4"
    @click.self="quickAdd.close()"
  >
    <div v-dialog
      ref="panel"
      role="dialog"
      aria-modal="true"
      aria-labelledby="quick-add-title"
      class="fixed inset-x-0 flex w-full flex-col overflow-hidden rounded-t-3xl border border-slate-800 bg-slate-900 shadow-2xl md:static md:max-h-[85dvh] md:max-w-md md:rounded-2xl"
      :style="panelStyle"
    >
      <div class="flex shrink-0 items-start justify-between gap-3 px-4 pt-4">
        <div class="min-w-0">
          <h2 id="quick-add-title" class="text-lg font-bold text-white">{{ $t('quickadd.quickAddSheet.quickAdd') }}</h2>
          <p class="truncate text-xs text-slate-400">{{ vehicle.name }}</p>
        </div>
        <button type="button" class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-slate-400 hover:bg-slate-800 hover:text-white" :aria-label="$t('common.close')" @click="quickAdd.close()">
          <X class="h-5 w-5" aria-hidden="true" />
        </button>
      </div>

      <div role="tablist" :aria-label="$t('quickadd.quickAddSheet.entryType')" class="mx-4 mt-3 flex shrink-0 gap-1 rounded-2xl border border-slate-800 bg-slate-950 p-1">
        <button
          v-for="t in tabs"
          :id="`quick-tab-${t.key}`"
          :key="t.key"
          type="button"
          role="tab"
          :aria-selected="activeKind === t.key"
          class="flex min-h-12 flex-1 items-center justify-center gap-1.5 whitespace-nowrap rounded-xl px-2 text-sm font-semibold transition-colors"
          :class="activeKind === t.key ? 'bg-rose-500/15 text-rose-200' : 'text-slate-400 hover:text-white'"
          @click="activeKind = t.key"
        >
          {{ $t(t.label) }}
          <span v-if="t.badge" class="rounded-full bg-amber-500/20 px-1.5 text-xs font-bold text-amber-300">{{ t.badge }}</span>
        </button>
      </div>

      <div role="tabpanel" :aria-labelledby="`quick-tab-${activeKind}`" class="flex min-h-0 flex-1 flex-col">
        <QuickPendingCosts
          v-if="activeKind === 'PENDING'"
          :vehicle-id="vehicle.id"
          :charges="pending"
          :total="pendingTotal"
          @completed="onCompleted"
        />
        <QuickChargeForm v-else-if="activeKind === 'CHARGE'" :vehicle="vehicle" @saved="onSaved" />
        <QuickFuelForm v-else-if="activeKind === 'FUEL'" :vehicle="vehicle" @saved="onSaved" />
        <QuickExpenseForm v-else :vehicle="vehicle" @saved="onSaved" />
      </div>
    </div>
  </div>

  <div
    v-if="toast && !quickAdd.isOpen"
    role="status"
    aria-live="polite"
    class="fixed bottom-[calc(5.5rem+env(safe-area-inset-bottom))] left-1/2 z-[55] flex w-[calc(100%-2rem)] max-w-sm -translate-x-1/2 items-center gap-2 rounded-xl border border-emerald-500/30 bg-slate-900 px-4 py-3 text-sm font-medium text-emerald-200 shadow-xl md:bottom-6"
  >
    <CheckCircle2 class="h-4 w-4 shrink-0" aria-hidden="true" />
    <span>{{ toast }}</span>
  </div>
</template>
