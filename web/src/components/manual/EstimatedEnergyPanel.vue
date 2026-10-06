<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Zap, CheckCircle2 } from 'lucide-vue-next'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import { currencySymbol, formatAmount } from '@/currency'
import { distanceUnit, formatDistance, formatPerDistanceValue, perDistance } from '@/units'

const props = defineProps<{
  vehicle: any
  canEdit: boolean
}>()

const router = useRouter()
const vehicleStore = useVehicleStore()
const { showAlert } = useConfirm()

const form = ref<{ kwh_100km: number | null; price_per_kwh: number | null }>({ kwh_100km: null, price_per_kwh: null })
const saving = ref(false)
const tco = ref<any | null>(null)

const preview = computed(() => {
  const kwh100 = Number(form.value.kwh_100km)
  const rate = Number(form.value.price_per_kwh)
  if (!kwh100 || !rate || kwh100 <= 0 || rate <= 0) return null
  const distance = tco.value?.estimated_energy_distance_km ?? (tco.value?.completeness?.untracked_distance_km || 0)
  if (distance <= 0) return null
  const kwh = (distance * kwh100) / 100
  return { distance, kwh, cost: kwh * rate }
})

function syncForm() {
  form.value = {
    kwh_100km: props.vehicle?.estimated_kwh_100km ?? null,
    price_per_kwh: props.vehicle?.estimated_price_per_kwh ?? null,
  }
}

async function loadTco() {
  tco.value = await api.getTCO(props.vehicle.id).catch(() => null)
}

async function save() {
  const kwh100 = form.value.kwh_100km != null ? Number(form.value.kwh_100km) : null
  const rate = form.value.price_per_kwh != null ? Number(form.value.price_per_kwh) : null
  if (kwh100 !== null && (kwh100 < 1 || kwh100 > 100)) {
    const min = perDistance(1).toLocaleString(intlLocale(), { maximumFractionDigits: 1 })
    const max = perDistance(100).toLocaleString(intlLocale(), { maximumFractionDigits: 1 })
    showAlert(t('manual.estimatedEnergyPanel.invalidConsumption', { unit: distanceUnit(), min, max }), t('manual.estimatedEnergyPanel.invalidField'), 'warning')
    return
  }
  if (rate !== null && (rate <= 0 || rate > 10)) {
    showAlert(t('manual.estimatedEnergyPanel.invalidRate', { cur: currencySymbol(vehicleStore.currency) }), t('manual.estimatedEnergyPanel.invalidField'), 'warning')
    return
  }
  saving.value = true
  try {
    await api.updateEstimatedEnergy(props.vehicle.id, {
      estimated_kwh_100km: kwh100,
      estimated_price_per_kwh: rate,
    })
    await vehicleStore.fetchVehicles()
    await loadTco()
    vehicleStore.lastSyncTimestamp = Date.now()
    showAlert(t('manual.estimatedEnergyPanel.saved'), t('common.success'), 'success')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    saving.value = false
  }
}

async function clear() {
  form.value = { kwh_100km: null, price_per_kwh: null }
  await save()
}

watch(() => props.vehicle?.id, () => {
  syncForm()
  loadTco()
})
onMounted(() => {
  syncForm()
  loadTco()
})
</script>

<template>
  <div class="space-y-5">
    <div class="bg-gradient-to-br from-info-950/40 to-slate-950/60 border border-info-500/20 rounded-xl p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="p-2 bg-info-500/10 text-info-400 rounded-lg">
            <Zap class="w-4 h-4" />
          </div>
          <div>
            <h4 class="text-xs font-bold text-info-400 uppercase tracking-wider">{{ $t('manual.estimatedEnergyPanel.energyEstimate') }}</h4>
            <p class="text-xs text-slate-400">{{ $t('manual.estimatedEnergyPanel.automaticallyFillInTheEnergy') }}</p>
          </div>
        </div>
        <span
          v-if="vehicle?.estimated_kwh_100km && vehicle?.estimated_price_per_kwh"
          class="text-xs px-2.5 py-0.5 rounded-full bg-success-500/10 text-success-400 border border-success-500/20 font-semibold flex items-center gap-1"
        >
          <CheckCircle2 class="w-3 h-3" /> {{ $t('manual.estimatedEnergyPanel.active') }}
        </span>
        <span v-else class="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-slate-700">{{ $t('manual.estimatedEnergyPanel.notConfigured') }}</span>
      </div>

      <form class="space-y-3" @submit.prevent="save">
        <div class="flex flex-wrap items-end gap-3">
          <div class="w-full sm:w-56">
            <label for="pre-tm-kwh" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.estimatedEnergyPanel.averageConsumptionKwh100km', { unit: distanceUnit() }) }}</label>
            <DistanceInput kind="per-distance"
              id="pre-tm-kwh"
              v-model="form.kwh_100km"
              step="0.1"
              min="1"
              max="100"
              :placeholder="$t('common.example', { value: formatPerDistanceValue(16.5, 1) })"
              :disabled="!canEdit"
              class="field"
            />
          </div>
          <div class="w-full sm:w-56">
            <label for="pre-tm-rate" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.estimatedEnergyPanel.electricityRateKwh', { cur: currencySymbol(vehicleStore.currency) }) }}</label>
            <NumberInput
              id="pre-tm-rate"
              v-model="form.price_per_kwh"
              min="0.01"
              max="5"
              :placeholder="$t('common.example', { value: $n(0.22) })"
              :disabled="!canEdit"
              class="field"
            />
          </div>
        </div>

        <div v-if="preview" class="bg-slate-900/80 border border-slate-800 rounded-xl p-3 text-xs space-y-1">
          <div class="text-slate-300 font-semibold flex items-center justify-between">
            <span>{{ $t('manual.estimatedEnergyPanel.estimateOverSmoothedKm', { distance: formatDistance(preview.distance) }) }}</span>
            <span class="text-info-400 font-bold font-mono">
              ≈ {{ formatAmount(preview.cost, vehicleStore.currency) }}
            </span>
          </div>
          <p class="text-slate-400 text-xs">
            {{ $t('manual.estimatedEnergyPanel.estimatedVolume') }}
            <strong class="text-slate-200 font-mono">{{ $t('manual.estimatedEnergyPanel.kwh', { kwh: Math.round(preview.kwh).toLocaleString(intlLocale()) }) }}</strong>
            {{ $t('manual.estimatedEnergyPanel.kmSpreadProRataAcross', { unit: distanceUnit(), cost: formatAmount(perDistance(preview.cost / (preview.distance || 1)), vehicleStore.currency, 3) }) }}
          </p>
        </div>

        <div v-if="canEdit" class="flex items-center justify-between pt-1">
          <button
            v-if="vehicle?.estimated_kwh_100km || vehicle?.estimated_price_per_kwh"
            type="button"
            class="tap-text text-xs text-slate-400 hover:text-rose-400 transition-colors"
            @click="clear"
          >
            {{ $t('manual.estimatedEnergyPanel.resetDisable') }}
          </button>
          <span v-else></span>

          <button
            type="submit"
            :disabled="saving"
            class="btn btn-lg btn-primary"
          >
            <Zap class="w-3.5 h-3.5" />
            <span>{{ saving ? $t('common.loading') : $t('manual.estimatedEnergyPanel.saveEstimate') }}</span>
          </button>
        </div>
      </form>
    </div>

    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex items-center justify-between gap-3 text-xs text-slate-300">
      <span>{{ $t('manual.estimatedEnergyPanel.aChargeMadeOutsideTeslamate') }}</span>
      <button
        type="button"
        class="shrink-0 px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 font-semibold rounded-xl"
        @click="router.push('/energy?tab=CHARGES')"
      >
        {{ $t('manual.estimatedEnergyPanel.chargesOutsideTeslamate') }}
      </button>
    </div>
  </div>
</template>
