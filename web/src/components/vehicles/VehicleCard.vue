<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { ref } from 'vue'
import { distanceUnit, formatDistance, formatDistanceValue, formatPerDistanceValue, perDistance } from '@/units'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import { Car, Plus, Trash2, Edit2, RefreshCw, CheckCircle2, AlertCircle, X, Gauge, Link2, FileText, Zap, Users, Pencil, Database } from 'lucide-vue-next'
import { canCharge, canRefuel, isElectricOnly, ownershipSummary, roleLabel } from '@/utils/vehicles'

// One vehicle of the list, with its contract summary and a TeslaMate connection test of its own
const props = defineProps<{ v: any; ownership: any | null }>()
const emit = defineEmits<{
  edit: [vehicle: any]
  delete: [vehicleId: string]
  members: [vehicle: any]
  ownership: [vehicle: any]
  dataSources: [vehicle: any]
}>()
const vehicleStore = useVehicleStore()

const cardTest = ref<{ loading: boolean; success?: boolean; status?: any; error?: string } | undefined>(undefined)

async function testCardConnection() {
  cardTest.value = { loading: true }
  try {
    const res = await api.testTeslaMate(props.v.id)
    cardTest.value = { loading: false, success: true, status: res.status }
  } catch (err: any) {
    cardTest.value = { loading: false, success: false, error: err.message }
  }
}

function clearCardTestResult() {
  cardTest.value = undefined
}
</script>

<template>
  <div
    class="bg-slate-900 border border-slate-800 p-5 rounded-2xl relative transition-all"
    :class="{ 'border-rose-500/40 shadow-lg shadow-rose-500/5': vehicleStore.activeVehicleId === v.id }"
  >
    <div class="flex items-start justify-between gap-2">
      <div class="flex min-w-0 items-center gap-3">
        <div class="shrink-0 p-3 bg-slate-800 rounded-xl text-rose-400">
          <Car class="w-6 h-6" />
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
            <h3 class="text-base font-bold text-white">{{ v.name }}</h3>
            <span
              v-if="v.role"
              class="px-2 py-0.5 text-xs font-semibold rounded-full uppercase tracking-wider"
              :class="{
                'bg-warning-500/10 text-warning-400 border border-warning-500/20': v.role === 'OWNER',
                'bg-info-500/10 text-info-400 border border-info-500/20': v.role === 'EDITOR',
                'bg-slate-700/50 text-slate-400 border border-slate-600/30': v.role === 'VIEWER',
              }"
            >
              {{ roleLabel(v.role) }}
            </span>
          </div>
          <p v-if="v.vin" class="break-all text-xs text-slate-400 font-mono">{{ v.vin }}</p>
        </div>
      </div>

      <div v-if="v.role === 'OWNER'" class="flex shrink-0 items-center gap-1">
        <button
          @click="emit('edit', v)"
          class="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
          :title="$t('common.edit')" :aria-label="$t('common.edit')"
        >
          <Edit2 class="w-4 h-4" />
        </button>
        <button
          @click="emit('delete', v.id)"
          class="p-2 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800"
          :title="$t('common.delete')" :aria-label="$t('common.delete')"
        >
          <Trash2 class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Telemetry & Stats -->
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-3 mt-4 pt-4 border-t border-slate-800 text-xs">
      <div>
        <span class="text-slate-400">{{ $t('vehicles.vehicleCard.currentOdometer') }}</span>
        <p class="text-sm font-bold text-slate-200 flex items-center gap-1.5 mt-0.5">
          <Gauge class="w-3.5 h-3.5 text-rose-400" />
          {{ formatDistance(v.current_odometer) }}
        </p>
      </div>

      <div v-if="v.role === 'OWNER'">
        <span class="text-slate-400 block mb-1">{{ $t('vehicles.vehicleCard.acquisitionAndContract') }}</span>
        <button
          type="button"
          @click="emit('ownership', v)"
          class="group text-left inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl text-xs font-semibold transition-all border"
          :class="
            ownership
              ? 'bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-300 border-indigo-500/30'
              : 'bg-warning-500/10 hover:bg-warning-500/20 text-warning-300 border-warning-500/30'
          "
          :title="ownership ? $t('vehicles.vehicleCard.editContract') : $t('vehicles.vehicleCard.setUpContract')"
        >
          <FileText v-if="ownership" class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
          <Plus v-else class="w-3.5 h-3.5 text-warning-400 shrink-0" />
          <span>{{ ownership ? ownershipSummary(ownership, v.currency || 'EUR') : $t('vehicles.vehicleCard.enterContract') }}</span>
          <Pencil v-if="ownership" class="w-3 h-3 text-indigo-400 opacity-60 group-hover:opacity-100 shrink-0 ml-0.5" />
        </button>
      </div>
      <div v-else>
        <span class="text-slate-400">{{ $t('vehicles.vehicleCard.vehicleAccess') }}</span>
        <p class="text-xs font-semibold text-slate-300 mt-1">
          {{ v.role === 'EDITOR' ? $t('vehicles.vehicleCard.editorRole') : $t('vehicles.vehicleCard.readOnly') }}
        </p>
      </div>

      <div v-if="canRefuel(v.powertrain)">
        <span class="text-slate-400">{{ $t('vehicles.vehicleCard.powertrain') }}</span>
        <p class="text-sm font-semibold mt-0.5 text-slate-200">{{ canCharge(v.powertrain) ? $t(`vehicles.powertrains.${v.powertrain}`) : $t('vehicles.vehicleCard.combustionFillUpsEnteredBy') }}</p>
      </div>
      <div v-if="canCharge(v.powertrain)">
        <span class="text-slate-400">{{ $t('vehicles.vehicleCard.teslamateConnection') }}</span>
        <p
          class="text-sm font-semibold mt-0.5"
          :class="
            cardTest?.success
              ? 'text-success-400'
              : cardTest?.error
              ? 'text-rose-400'
              : v.teslamate_api_url
              ? 'text-success-400/80'
              : 'text-slate-400'
          "
        >
          {{
            v.role === 'OWNER'
              ? (cardTest?.success
                ? $t('vehicles.vehicleCard.online')
                : cardTest?.error
                ? $t('vehicles.vehicleCard.connectionError')
                : v.teslamate_api_url
                ? $t('vehicles.vehicleCard.configured')
                : $t('vehicles.vehicleCard.notConfigured'))
              : $t('vehicles.vehicleCard.managedByAdmin')
          }}
        </p>
      </div>

      <div v-if="isElectricOnly(v.powertrain)">
        <span class="text-slate-400">{{ $t('vehicles.vehicleCard.estimatedEnergy') }}</span>
        <p v-if="v.estimated_kwh_100km && v.estimated_price_per_kwh" class="text-xs font-semibold text-info-400 flex items-center gap-1 mt-1">
          <Zap class="w-3.5 h-3.5 text-info-400" />
          {{ $t('vehicles.vehicleCard.kwh100kmKwh', { unit: distanceUnit(), estimated_kwh_100km: formatPerDistanceValue(Number(v.estimated_kwh_100km)), price: `${formatAmount(v.estimated_price_per_kwh, v.currency || 'EUR', 3)}/kWh` }) }}
        </p>
        <p v-else class="text-xs text-slate-400 mt-1">{{ $t('vehicles.vehicleCard.notConfigured') }}</p>
      </div>
    </div>

    <!-- Actions -->
    <div class="mt-4 pt-3 flex items-center justify-between gap-2 flex-wrap">
      <button
        @click="emit('members', v)"
        class="btn btn-secondary"
        :title="$t('vehicles.vehicleCard.manageAccessAndCoDrivers')"
      >
        <Users class="w-3.5 h-3.5 text-violet-400" />
        <span>{{ $t('vehicles.vehicleCard.sharingAndAccess') }}</span>
      </button>
      <button
        v-if="v.role === 'OWNER'"
        @click="emit('ownership', v)"
        class="btn btn-secondary"
      >
        <FileText class="w-3.5 h-3.5 text-indigo-400" />
        <span>{{ $t('vehicles.vehicleCard.acquisitionAndFinancing') }}</span>
      </button>
      <button
        @click="emit('dataSources', v)"
        class="btn btn-secondary"
      >
        <Database class="w-3.5 h-3.5 text-cyan-400" />
        <span>{{ $t('vehicles.vehicleCard.dataSources') }}</span>
      </button>
      <router-link
        to="/manual"
        @click="vehicleStore.setActiveVehicle(v.id)"
        class="btn btn-secondary tap-text"
        :title="$t('vehicles.vehicleCard.odometerReadingsAndManualEntries')"
      >
        <Gauge class="w-3.5 h-3.5 text-cyan-400" />
        <span>{{ $t('vehicles.vehicleCard.manualTracking') }}</span>
      </router-link>
      <button
        v-if="v.role === 'OWNER' && v.teslamate_api_url"
        @click="testCardConnection()"
        :disabled="cardTest?.loading"
        class="btn btn-secondary"
      >
        <RefreshCw v-if="cardTest?.loading" class="w-3.5 h-3.5 animate-spin text-rose-400" />
        <Link2 v-else class="w-3.5 h-3.5" />
        <span>{{ cardTest?.loading ? $t('vehicles.vehicleCard.testing') : $t('vehicles.vehicleCard.testApi') }}</span>
      </button>

      <button
        v-if="vehicleStore.activeVehicleId !== v.id"
        @click="vehicleStore.setActiveVehicle(v.id)"
        class="ml-auto px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-xs text-rose-400 font-semibold rounded-lg"
      >
        {{ $t('vehicles.vehicleCard.select') }}
      </button>
      <span v-else class="ml-auto text-xs font-semibold text-rose-400 px-3 py-1.5 bg-rose-500/10 rounded-lg">
        {{ $t('vehicles.vehicleCard.activeVehicle') }}
      </span>
    </div>

    <!-- Test feedback per card -->
    <div
      v-if="cardTest && !cardTest.loading"
      class="mt-3 p-3 rounded-xl text-xs flex items-start justify-between gap-2"
      :class="cardTest.success ? 'bg-success-500/10 text-success-300 border border-success-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20'"
    >
      <div class="flex items-start gap-2">
        <CheckCircle2 v-if="cardTest.success" class="w-4 h-4 shrink-0 text-success-400 mt-0.5" />
        <AlertCircle v-else class="w-4 h-4 shrink-0 text-danger-400 mt-0.5" />
        <div>
          <span v-if="cardTest.success">
            {{ $t('vehicles.testSuccess', { unit: distanceUnit(), state: cardTest.status?.state || $t('vehicles.vehicleCard.online'), odometer: formatDistanceValue(cardTest.status?.odometer || 0) }) }}
          </span>
          <span v-else>{{ cardTest.error }}</span>
        </div>
      </div>
      <button @click="clearCardTestResult()" class="text-slate-400 hover:text-white p-0.5">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>
