<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import {
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  X,
  Link2,
  ChevronDown,
  ChevronRight,
  Zap,
  Flame,
  Copy,
  Sliders,
} from 'lucide-vue-next'
import { emptyVehicleForm, vehicleFormFrom } from '@/utils/vehicles'
import { CURRENCIES } from '@/utils/expenses'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistanceValue, formatPerDistanceValue } from '@/units'
import { useVehicleStore } from '@/stores/vehicle'

// Adds a vehicle, or edits \`editing\`.
const props = withDefaults(
  defineProps<{
    editing?: any | null
    vehicles?: any[]
  }>(),
  {
    editing: null,
    vehicles: undefined,
  }
)
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const vehicleStore = useVehicleStore()
const existingVehicles = computed(() => props.vehicles ?? vehicleStore.vehicles)

const isEditing = computed(() => !!props.editing)
const editingId = computed(() => props.editing?.id ?? null)
const modalTestLoading = ref(false)
const modalTestResult = ref<{ success: boolean; status?: any; error?: string } | null>(null)
const form = ref(emptyVehicleForm(existingVehicles.value))
const showAdvanced = ref(false)
// Shows the TeslaMate fields; without a teslamateapi URL the server tracks the vehicle as manual
const connectTeslaMate = ref(false)

const tariffPlans = ref<any[]>([])

// Existing vehicles that already have a telemetry API URL configured
const existingConnectedVehicles = computed(() => {
  return existingVehicles.value.filter(
    (v: any) => v.teslamate_api_url && (!props.editing || v.id !== props.editing.id)
  )
})

function copyConnectionFrom(source: any) {
  form.value.teslamate_api_url = source.teslamate_api_url || ''
  form.value.teslamate_grafana_url = source.teslamate_grafana_url || ''
  form.value.teslamate_auth_type = source.teslamate_auth_type || 'NONE'
  form.value.teslamate_basic_user = source.teslamate_basic_user || ''
}

function setPowertrain(p: 'EV' | 'ICE') {
  if (isEditing.value) return
  form.value.powertrain = p
  if (p === 'ICE') {
    connectTeslaMate.value = false
  }
}

async function loadTariffPlans() {
  try {
    const res = await api.getTariffPlans()
    tariffPlans.value = res.plans || []
  } catch (err) {
    console.error('Failed to load tariff plans', err)
  }
}

watch(open, (isOpen) => {
  if (!isOpen) return
  modalTestResult.value = null
  showAdvanced.value = false
  form.value = props.editing ? vehicleFormFrom(props.editing) : emptyVehicleForm(existingVehicles.value)
  connectTeslaMate.value = !!form.value.teslamate_api_url
  loadTariffPlans()
})

async function handleSave() {
  try {
    const payload: Record<string, any> = { ...form.value }
    if (payload.powertrain === 'ICE') {
      payload.teslamate_car_id = null
      payload.teslamate_api_url = ''
      payload.teslamate_grafana_url = ''
      payload.teslamate_api_key = ''
      payload.teslamate_basic_user = ''
      payload.teslamate_basic_pass = ''
      payload.tariff_plan_id = null
      payload.is_home_charger_default = false
      payload.estimated_kwh_100km = null
      payload.estimated_price_per_kwh = null
    } else if (!connectTeslaMate.value) {
      payload.teslamate_car_id = null
      payload.teslamate_api_url = ''
      payload.teslamate_grafana_url = ''
      payload.teslamate_api_key = ''
      payload.teslamate_basic_user = ''
      payload.teslamate_basic_pass = ''
    } else {
      payload.teslamate_car_id = payload.teslamate_car_id || null
    }

    if (isEditing.value && editingId.value) {
      await api.updateVehicle(editingId.value, payload)
    } else {
      await api.createVehicle(payload)
    }
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

async function testModalConnection() {
  if (!form.value.teslamate_api_url) {
    modalTestResult.value = { success: false, error: t('vehicles.vehicleFormModal.urlFirst') }
    return
  }
  modalTestLoading.value = true
  modalTestResult.value = null
  try {
    const res = await api.testTeslaMateRaw(form.value)
    modalTestResult.value = { success: true, status: res.status }
  } catch (err: any) {
    modalTestResult.value = { success: false, error: err.message }
  } finally {
    modalTestLoading.value = false
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white">{{ isEditing ? $t('vehicles.vehicleFormModal.edit') : $t('shell.topBar.addAVehicle') }}</h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="vehicle-modal-form" @submit.prevent="handleSave" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <!-- 1. Identité du véhicule : Marque & Modèle (100% Free text according to CLAUDE.md) -->
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="vehicle-make" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('onboarding.onboardingView.make') }}</label>
            <input
              id="vehicle-make"
              v-model="form.make"
              :placeholder="$t('onboarding.onboardingView.makePlaceholder')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>
          <div>
            <label for="vehicle-model" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('onboarding.onboardingView.model') }}</label>
            <input
              id="vehicle-model"
              v-model="form.model"
              :placeholder="$t('onboarding.onboardingView.modelPlaceholder')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>
        </div>

        <!-- Nom usuel -->
        <div>
          <label for="vehicle-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.vehicleName') }}</label>
          <input
            id="vehicle-name"
            v-model="form.name"
            required
            :placeholder="$t('vehicles.vehicleFormModal.eGMyCar')"
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500 transition-colors"
          />
        </div>

        <!-- Motorisation (Boutons visuels au lieu d'un simple select) -->
        <div>
          <label class="block text-xs font-semibold text-slate-300 mb-1.5">{{ $t('vehicles.vehicleFormModal.powertrain') }}</label>
          <div class="grid grid-cols-2 gap-2.5">
            <button
              type="button"
              :disabled="isEditing"
              @click="setPowertrain('EV')"
              class="p-2.5 rounded-xl border flex items-center gap-2.5 transition-all text-left disabled:opacity-50 disabled:cursor-not-allowed"
              :class="form.powertrain === 'EV' ? 'bg-rose-500/10 border-rose-500/50 text-white shadow-sm ring-1 ring-rose-500/20' : 'bg-slate-800/40 border-slate-700/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'"
            >
              <Zap class="w-4 h-4 text-amber-400 shrink-0" />
              <span class="text-xs font-semibold leading-tight">{{ $t('onboarding.onboardingView.electricTeslamateTrackingAvailable') }}</span>
            </button>
            <button
              type="button"
              :disabled="isEditing"
              @click="setPowertrain('ICE')"
              class="p-2.5 rounded-xl border flex items-center gap-2.5 transition-all text-left disabled:opacity-50 disabled:cursor-not-allowed"
              :class="form.powertrain === 'ICE' ? 'bg-rose-500/10 border-rose-500/50 text-white shadow-sm ring-1 ring-rose-500/20' : 'bg-slate-800/40 border-slate-700/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'"
            >
              <Flame class="w-4 h-4 text-orange-400 shrink-0" />
              <span class="text-xs font-semibold leading-tight">{{ $t('onboarding.onboardingView.combustionFillUpsEnteredBy') }}</span>
            </button>
          </div>
        </div>

        <!-- Devise & Kilométrage -->
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="vehicle-current-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ form.powertrain === 'ICE' ? $t('vehicles.vehicleFormModal.currentMileage', { unit: distanceUnit() }) : $t('vehicles.vehicleFormModal.initialOdometer', { unit: distanceUnit() }) }}</label>
            <DistanceInput
              id="vehicle-current-odometer"
              v-model="form.current_odometer"
              step="1"
              :disabled="!!form.teslamate_api_url && connectTeslaMate"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:border-rose-500"
            />
          </div>
          <div>
            <label for="vehicle-currency" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.currency') }}</label>
            <select
              id="vehicle-currency"
              v-model="form.currency"
              :disabled="isEditing"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white disabled:opacity-50 focus:outline-none focus:border-rose-500"
            >
              <option v-for="c in CURRENCIES" :key="c" :value="c">{{ c }}</option>
            </select>
            <p v-if="isEditing" class="mt-1 text-[11px] text-slate-500">{{ $t('vehicles.vehicleFormModal.currencyFixed') }}</p>
          </div>
        </div>

        <!-- VIN Optionnel -->
        <div>
          <label for="vehicle-vin" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.vinOptional') }}</label>
          <input
            id="vehicle-vin"
            v-model="form.vin"
            placeholder="VIN"
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500 transition-colors"
          />
        </div>

        <!-- 2. TeslaMate synchronization (electric vehicles); the tracking mode follows from it on the server -->
        <div v-if="form.powertrain === 'EV'" class="pt-3 border-t border-slate-800">
          <label
            for="vehicle-connect-teslamate"
            class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all"
            :class="connectTeslaMate
              ? 'bg-rose-500/10 border-rose-500/50 shadow-sm ring-1 ring-rose-500/20'
              : 'bg-slate-800/40 border-slate-700/60 hover:bg-slate-800'"
          >
            <input id="vehicle-connect-teslamate" v-model="connectTeslaMate" type="checkbox" class="mt-0.5 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-900 border-slate-700" />
            <div>
              <span class="text-xs font-semibold text-white block">{{ $t('vehicles.vehicleFormModal.teslamateSync') }}</span>
              <span class="text-[11px] text-slate-400 block mt-0.5">{{ $t('vehicles.vehicleFormModal.teslamateSyncDesc') }}</span>
            </div>
          </label>
        </div>

        <!-- 3. Paramètres de télémétrie connectée (Visible UNIQUEMENT si EV et CONNECTED) -->
        <div v-if="form.powertrain === 'EV' && connectTeslaMate" class="pt-3 border-t border-slate-800 space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-rose-400 uppercase tracking-wider">{{ $t('vehicles.vehicleFormModal.telemetrySettings') }}</h4>
            <span class="text-[10px] px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-400 border border-rose-500/20">TeslaMate</span>
          </div>

          <!-- Bouton de réutilisation pratique depuis un autre véhicule connecté -->
          <div v-if="existingConnectedVehicles.length > 0 && !isEditing" class="p-2.5 rounded-xl bg-slate-800/60 border border-slate-700/60 flex flex-wrap items-center gap-2 text-xs">
            <span class="text-slate-400 text-[11px]">{{ $t('vehicles.vehicleFormModal.copyFromExisting', { name: '' }) }}:</span>
            <button
              v-for="ev in existingConnectedVehicles"
              :key="ev.id"
              type="button"
              @click="copyConnectionFrom(ev)"
              class="px-2 py-1 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg text-xs font-medium flex items-center gap-1 transition-colors"
            >
              <Copy class="w-3 h-3 text-rose-400" />
              {{ ev.name || ev.model || 'Véhicule' }}
            </button>
          </div>

          <div>
            <label for="vehicle-teslamate-api-url" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.teslamateapiBaseUrl') }}</label>
            <input
              id="vehicle-teslamate-api-url"
              v-model="form.teslamate_api_url"
              :placeholder="$t('vehicles.vehicleFormModal.eGHttp1921682')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
            />
          </div>

          <div>
            <label for="vehicle-teslamate-grafana-url" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.teslamateGrafanaUrlOptional') }}</label>
            <input
              id="vehicle-teslamate-grafana-url"
              v-model="form.teslamate_grafana_url"
              type="url"
              :placeholder="$t('vehicles.vehicleFormModal.eGHttp192168')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
            />
            <p class="text-[11px] text-slate-500 mt-1">{{ $t('vehicles.vehicleFormModal.addsAnOpenInTeslamate') }}</p>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-teslamate-car-id" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.carIdInTeslamate') }}</label>
              <input
                id="vehicle-teslamate-car-id"
                v-model.number="form.teslamate_car_id"
                type="number"
                min="1"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
              />
            </div>
            <div>
              <label for="vehicle-teslamate-auth-type" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.authenticationMode') }}</label>
              <select
                id="vehicle-teslamate-auth-type"
                v-model="form.teslamate_auth_type"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
              >
                <option value="NONE">{{ $t('vehicles.vehicleFormModal.noneLan') }}</option>
                <option value="BEARER">{{ $t('vehicles.vehicleFormModal.bearerTokenApiToken') }}</option>
                <option value="BASIC">{{ $t('vehicles.vehicleFormModal.httpBasicAuth') }}</option>
              </select>
            </div>
          </div>

          <div v-if="form.teslamate_auth_type === 'BEARER'">
            <label for="vehicle-teslamate-api-key" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.teslamateApiKeyToken') }}</label>
            <input
              id="vehicle-teslamate-api-key"
              v-model="form.teslamate_api_key"
              type="password"
              placeholder="••••••••"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
            />
          </div>

          <div v-if="form.teslamate_auth_type === 'BASIC'" class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-teslamate-basic-user" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.basicAuthUser') }}</label>
              <input
                id="vehicle-teslamate-basic-user"
                v-model="form.teslamate_basic_user"
                placeholder="admin"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
              />
            </div>
            <div>
              <label for="vehicle-teslamate-basic-pass" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.basicAuthPassword') }}</label>
              <input
                id="vehicle-teslamate-basic-pass"
                v-model="form.teslamate_basic_pass"
                type="password"
                placeholder="••••••••"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500"
              />
            </div>
          </div>

          <!-- Test de connexion -->
          <div v-if="form.teslamate_api_url" class="pt-2">
            <button
              type="button"
              @click="testModalConnection"
              :disabled="modalTestLoading"
              class="w-full py-2 px-3 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center justify-center gap-2 border border-slate-700 disabled:opacity-50 transition-colors"
            >
              <RefreshCw v-if="modalTestLoading" class="w-3.5 h-3.5 animate-spin text-rose-400" />
              <Link2 v-else class="w-3.5 h-3.5 text-rose-400" />
              <span>{{ modalTestLoading ? $t('vehicles.vehicleFormModal.testing') : $t('vehicles.vehicleFormModal.testConnection') }}</span>
            </button>

            <div
              v-if="modalTestResult"
              class="mt-2.5 p-3 rounded-xl text-xs flex items-start gap-2"
              :class="modalTestResult.success ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20'"
            >
              <CheckCircle2 v-if="modalTestResult.success" class="w-4 h-4 shrink-0 text-emerald-400 mt-0.5" />
              <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
              <div class="flex-1">
                <div v-if="modalTestResult.success">
                  <strong class="font-semibold">{{ $t('vehicles.vehicleFormModal.connectionSuccessful') }}</strong>
                  <p class="text-[11px] text-emerald-200/80 mt-0.5">
                    {{ $t('vehicles.vehicleFormModal.testStatus', { unit: distanceUnit(), state: modalTestResult.status?.state || $t('vehicles.vehicleCard.online'), odometer: formatDistanceValue(modalTestResult.status?.odometer || 0) }) }}
                  </p>
                </div>
                <div v-else>
                  <strong class="font-semibold">{{ $t('vehicles.vehicleFormModal.connectionFailed') }}</strong>
                  <p class="text-[11px] text-rose-200/90 mt-0.5">{{ modalTestResult.error }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 4. Options complémentaires (Repliables pour ne pas encombrer la création) -->
        <div v-if="form.powertrain === 'EV'" class="pt-3 border-t border-slate-800">
          <button
            type="button"
            @click="showAdvanced = !showAdvanced"
            class="w-full flex items-center justify-between py-2 text-xs font-semibold text-slate-300 hover:text-white transition-colors"
          >
            <span class="flex items-center gap-2">
              <Sliders class="w-4 h-4 text-amber-400" />
              {{ $t('vehicles.vehicleFormModal.advancedOptions') }}
            </span>
            <ChevronDown v-if="showAdvanced" class="w-4 h-4 text-slate-400" />
            <ChevronRight v-else class="w-4 h-4 text-slate-400" />
          </button>
          <p class="text-[11px] text-slate-400 mb-2">{{ $t('vehicles.vehicleFormModal.advancedOptionsDesc') }}</p>

          <div v-if="showAdvanced" class="space-y-3.5 pt-2">
            <!-- Tarifs & Borne -->
            <div>
              <label for="vehicle-tariff-plan" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('tariffs.planSelectLabel') }}</label>
              <select id="vehicle-tariff-plan" v-model="form.tariff_plan_id" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500">
                <option :value="null">{{ $t('tariffs.noPlanManual') }}</option>
                <option v-for="p in tariffPlans" :key="p.id" :value="p.id">
                  {{ p.name }} ({{ p.plan_type }})
                </option>
              </select>
              <p class="text-[11px] text-slate-400 mt-1">{{ $t('tariffs.planSelectHint') }}</p>
            </div>

            <label for="vehicle-home-charger-default" class="flex items-start gap-2.5 p-3 rounded-xl bg-slate-800/40 border border-slate-700/60 cursor-pointer">
              <input id="vehicle-home-charger-default" type="checkbox" v-model="form.is_home_charger_default" class="mt-0.5 rounded border-slate-600 text-rose-600 focus:ring-rose-500 bg-slate-900" />
              <div class="text-xs">
                <span class="font-semibold text-white block">{{ $t('vehicles.homeChargerDefaultLabel') }}</span>
                <span class="text-slate-400 block mt-0.5">{{ $t('vehicles.homeChargerDefaultHint') }}</span>
              </div>
            </label>

            <!-- Consommation et tarif estimé -->
            <div class="grid grid-cols-2 gap-3 pt-1">
              <div>
                <label for="vehicle-pre-kwh" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.consumptionKwh100km', { unit: distanceUnit() }) }}</label>
                <DistanceInput kind="per-distance" id="vehicle-pre-kwh" v-model="form.estimated_kwh_100km" step="0.1" min="1" max="100" :placeholder="$t('common.example', { value: formatPerDistanceValue(16.5, 1) })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
              </div>
              <div>
                <label for="vehicle-pre-rate" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.rateKwh', { currency: form.currency }) }}</label>
                <input id="vehicle-pre-rate" v-model.number="form.estimated_price_per_kwh" type="number" step="0.0001" min="0.01" max="5" :placeholder="$t('common.example', { value: $n(0.22) })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
              </div>
            </div>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="vehicle-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors shadow-lg shadow-rose-600/20">
          {{ $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
