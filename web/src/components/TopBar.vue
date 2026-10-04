<script setup lang="ts">
import { computed } from 'vue'
import { formatDistance } from '@/units'
import { intlLocale } from '@/i18n'
import { useVehicleStore } from '@/stores/vehicle'
import { useOfflineStore } from '@/stores/offline'
import { RefreshCw, Car, Gauge, Plus, AlertCircle, AlertTriangle, X, CheckCircle2, WifiOff, CloudUpload } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import { t } from '@/i18n'
import { apiMessageText } from '@/services/apiError'

const vehicleStore = useVehicleStore()
const offlineStore = useOfflineStore()
const router = useRouter()

const syncSummary = computed(() => {
  const res = vehicleStore.syncResult
  if (!res) return ''
  const drives = res.drives_added !== undefined ? res.drives_added : res.drives_synced
  const charges = res.charges_added !== undefined ? res.charges_added : res.charges_synced
  const parts: string[] = []
  if (drives > 0) parts.push(t('shell.topBar.driveCount', drives))
  if (charges > 0) parts.push(t('shell.topBar.chargeCount', charges))
  if (parts.length > 0) {
    return parts.join(', ')
  }
  return t('shell.topBar.upToDate')
})

function onVehicleChange(event: Event) {
  const target = event.target as HTMLSelectElement
  if (target.value === 'new') {
    router.push({ path: '/vehicles', query: { add: '1' } })
  } else {
    vehicleStore.setActiveVehicle(target.value)
  }
}
</script>

<template>
  <div>
    <header class="bg-slate-900/60 backdrop-blur-md border-b border-slate-800 sticky top-0 z-40 px-3 sm:px-4 py-3 flex items-center justify-between gap-2">
      <!-- Vehicle Selector -->
      <div class="flex min-w-0 flex-1 items-center gap-3">
        <div class="hidden sm:block p-2 bg-slate-800 rounded-lg text-slate-400">
          <Car class="w-5 h-5" />
        </div>

        <div v-if="vehicleStore.vehicles.length" class="flex min-w-0 flex-1 items-center gap-2">
          <label for="topbar-active-vehicle" class="sr-only">{{ $t('shell.topBar.activeVehicle') }}</label>
          <select id="topbar-active-vehicle"
            :value="vehicleStore.activeVehicle?.id"
            @change="onVehicleChange"
            class="field font-semibold flex-1 sm:flex-none max-w-[220px] md:max-w-xs truncate"
          >
            <option v-for="v in vehicleStore.vehicles" :key="v.id" :value="v.id">
              {{ v.name }}
            </option>
            <option value="new">{{ $t('shell.topBar.addAVehicle2') }}</option>
          </select>

          <!-- Odometer pill -->
          <div class="hidden min-[360px]:flex shrink-0 items-center gap-1.5 bg-slate-800/80 px-2 py-1 rounded-lg text-xs text-slate-300 border border-slate-700/50">
            <Gauge class="w-3.5 h-3.5 text-rose-400 shrink-0" />
            <span class="truncate max-w-[90px] sm:max-w-none">{{ formatDistance(vehicleStore.activeVehicle?.current_odometer || 0) }}</span>
          </div>
        </div>

        <div v-else-if="!vehicleStore.isInitialized || vehicleStore.isLoading" class="flex items-center gap-2">
          <div class="w-4 h-4 border-2 border-rose-500 border-t-transparent rounded-full animate-spin"></div>
          <span class="text-xs text-slate-400">{{ $t('common.loading') }}</span>
        </div>

        <div v-else>
          <button
            @click="router.push('/vehicles')"
            class="text-xs bg-rose-600 hover:bg-rose-500 text-white font-medium px-3 py-1.5 rounded-lg flex items-center gap-1.5 transition-colors"
          >
            <Plus class="w-3.5 h-3.5" />
            {{ $t('shell.topBar.addAVehicle') }}
          </button>
        </div>
      </div>

      <!-- Actions (Sync) -->
      <div class="flex shrink-0 items-center gap-2">
        <!-- Offline queue -->
        <div
          v-if="!offlineStore.isOnline || offlineStore.pendingCount > 0"
          class="flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-lg border"
          :class="offlineStore.isOnline ? 'text-info-300 bg-info-500/10 border-info-500/20' : 'text-warning-300 bg-warning-500/10 border-warning-500/20'"
          :title="offlineStore.isOnline ? $t('shell.topBar.offlineSendingTitle') : $t('shell.topBar.offlineKeptTitle')"
          :aria-label="`${!offlineStore.isOnline ? $t('shell.topBar.offlineSentence') + ' ' : ''}${offlineStore.pendingCount > 0 ? $t('shell.topBar.pendingEntries', { count: offlineStore.pendingCount }) : ''}`"
        >
          <WifiOff v-if="!offlineStore.isOnline" class="w-3.5 h-3.5 shrink-0" />
          <CloudUpload v-else class="w-3.5 h-3.5 shrink-0" :class="{ 'animate-pulse': offlineStore.isFlushing }" />
          <span v-if="!offlineStore.isOnline" class="hidden sm:inline">{{ $t('shell.topBar.offline') }}</span>
          <span v-if="offlineStore.pendingCount > 0">{{ offlineStore.pendingCount }}<span class="hidden sm:inline"> {{ $t('shell.topBar.pending') }}</span></span>
        </div>

        <button
          v-if="vehicleStore.hasTeslaMate"
          @click="vehicleStore.syncActiveVehicle"
          :disabled="vehicleStore.isSyncing"
          class="flex min-h-11 min-w-11 items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg transition-all sm:min-h-0 sm:min-w-0"
          :class="
            vehicleStore.isSyncing
              ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30'
              : 'bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700'
          "
          :title="$t('shell.topBar.syncWithTeslamate')"
        >
          <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': vehicleStore.isSyncing }" />
          <span class="hidden sm:inline">{{ vehicleStore.isSyncing ? $t('shell.topBar.syncing') : $t('shell.topBar.sync') }}</span>
        </button>

        <!-- Sync result success -->
        <div
          v-if="vehicleStore.syncResult && !vehicleStore.syncResult.warnings?.length"
          class="hidden md:flex items-center gap-1.5 text-xs text-success-400 bg-success-500/10 border border-success-500/20 px-2.5 py-1 rounded-lg"
        >
          <CheckCircle2 class="w-3 h-3 shrink-0" />
          <span>{{ syncSummary }}</span>
          <button @click="vehicleStore.clearSyncStatus" class="ml-1 text-slate-400 hover:text-slate-200">
            <X class="w-3 h-3" />
          </button>
        </div>
      </div>
    </header>

    <!-- Offline confirmation -->
    <div
      v-if="offlineStore.lastQueuedLabel"
      class="bg-info-500/15 border-b border-info-500/30 px-4 py-2 text-xs text-info-200 flex items-center gap-2"
    >
      <CloudUpload class="w-4 h-4 shrink-0 text-info-400" />
      <span><strong>{{ offlineStore.lastQueuedLabel }}</strong> {{ $t('shell.topBar.savedOfflineItWillBe') }}</span>
    </div>

    <!-- Offline replay failures -->
    <div
      v-if="offlineStore.failures.length"
      class="bg-rose-500/15 border-b border-rose-500/30 px-4 py-2.5 text-xs text-rose-300 flex items-center justify-between gap-3"
    >
      <div class="flex items-start gap-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-danger-400" />
        <span>
          <strong>{{ $t('shell.topBar.offlineEntriesRejected') }}</strong>
          {{ offlineStore.failures.map((f) => `${f.label} (${f.error})`).join(' ; ') }}
        </span>
      </div>
      <button @click="offlineStore.dismissFailures" class="tap text-danger-400 hover:text-white p-1 rounded transition-colors">
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Error Banner for Sync Failure -->
    <div
      v-if="vehicleStore.syncError"
      class="bg-rose-500/15 border-b border-rose-500/30 px-4 py-2.5 text-xs text-rose-300 flex items-center justify-between gap-3"
    >
      <div class="flex items-center gap-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-danger-400" />
        <span><strong>{{ $t('shell.topBar.syncError') }}</strong> {{ vehicleStore.syncError }}</span>
      </div>
      <button
        @click="vehicleStore.clearSyncStatus"
        class="tap text-rose-400 hover:text-white p-1 rounded transition-colors"
      >
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- Warning Banner for Partial Sync -->
    <div
      v-if="vehicleStore.syncResult?.warnings?.length"
      class="bg-warning-500/15 border-b border-warning-500/30 px-4 py-2.5 text-xs text-warning-300 flex items-center justify-between gap-3"
    >
      <div class="flex items-center gap-2">
        <AlertTriangle class="w-4 h-4 shrink-0 text-warning-400" />
        <span>
          <strong>{{ $t('shell.topBar.syncFinishedWithWarnings') }}</strong> {{ syncSummary }}.
          <span class="text-warning-200/80">({{ vehicleStore.syncResult.warnings.map(apiMessageText).join(' ; ') }})</span>
        </span>
      </div>
      <button
        @click="vehicleStore.clearSyncStatus"
        class="tap text-warning-400 hover:text-white p-1 rounded transition-colors"
      >
        <X class="w-4 h-4" />
      </button>
    </div>
  </div>
</template>
