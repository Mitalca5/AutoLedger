<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { RefreshCw, CheckCircle2, AlertCircle, X, Link2 } from 'lucide-vue-next'
import { emptyVehicleForm, vehicleFormFrom } from '@/utils/vehicles'
import { CURRENCIES } from '@/utils/expenses'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistanceValue, formatPerDistanceValue } from '@/units'

// Adds a vehicle, or edits \`editing\`. The TeslaMate connection can be tested with the values typed so far.
const props = defineProps<{ editing: any | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const isEditing = computed(() => !!props.editing)
const editingId = computed(() => props.editing?.id ?? null)
const modalTestLoading = ref(false)
const modalTestResult = ref<{ success: boolean; status?: any; error?: string } | null>(null)
const form = ref(emptyVehicleForm())

const tariffPlans = ref<any[]>([])

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
  form.value = props.editing ? vehicleFormFrom(props.editing) : emptyVehicleForm()
  loadTariffPlans()
})

async function handleSave() {
  try {
    if (isEditing.value && editingId.value) {
      await api.updateVehicle(editingId.value, form.value)
    } else {
      await api.createVehicle(form.value)
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

      <form id="vehicle-modal-form" @submit.prevent="handleSave" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-3.5">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="vehicle-make" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('onboarding.onboardingView.make') }}</label>
            <input id="vehicle-make" v-model="form.make" :placeholder="$t('onboarding.onboardingView.makePlaceholder')" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label for="vehicle-model" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('onboarding.onboardingView.model') }}</label>
            <input id="vehicle-model" v-model="form.model" :placeholder="$t('onboarding.onboardingView.modelPlaceholder')" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
        </div>

        <div>
          <label for="vehicle-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.vehicleName') }}</label>
          <input id="vehicle-name" v-model="form.name" required :placeholder="$t('vehicles.vehicleFormModal.eGMyCar')" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
        </div>

        <div>
          <label for="vehicle-powertrain" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.powertrain') }}</label>
          <select id="vehicle-powertrain" v-model="form.powertrain" :disabled="isEditing" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white disabled:opacity-50">
            <option value="EV">{{ $t('vehicles.vehicleFormModal.electricTeslamateTrackingAvailable') }}</option>
            <option value="ICE">{{ $t('vehicles.vehicleFormModal.combustionFillUpsEnteredBy') }}</option>
          </select>
        </div>

        <div>
          <label for="vehicle-currency" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.currency') }}</label>
          <select id="vehicle-currency" v-model="form.currency" :disabled="isEditing" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white disabled:opacity-50">
            <option v-for="c in CURRENCIES" :key="c" :value="c">{{ c }}</option>
          </select>
          <p v-if="isEditing" class="mt-1 text-[11px] text-slate-500">{{ $t('vehicles.vehicleFormModal.currencyFixed') }}</p>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="vehicle-vin" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.vinOptional') }}</label>
            <input id="vehicle-vin" v-model="form.vin" placeholder="VIN" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>
          <div>
            <label for="vehicle-current-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ form.powertrain === 'ICE' ? $t('vehicles.vehicleFormModal.currentMileage', { unit: distanceUnit() }) : $t('vehicles.vehicleFormModal.initialOdometer', { unit: distanceUnit() }) }}</label>
            <DistanceInput
              id="vehicle-current-odometer"
              v-model="form.current_odometer"
              step="1"
              :disabled="!!form.teslamate_api_url"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white disabled:opacity-50 disabled:cursor-not-allowed"
            />
          </div>
        </div>

        <!-- Energy estimate section -->
        <div v-if="form.powertrain !== 'ICE'" class="pt-2 border-t border-slate-800 space-y-3">
          <h4 class="text-xs font-bold text-sky-400 uppercase tracking-wider">{{ $t('vehicles.vehicleFormModal.estimatedEnergyOptional') }}</h4>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-pre-kwh" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.consumptionKwh100km', { unit: distanceUnit() }) }}</label>
              <DistanceInput kind="per-distance" id="vehicle-pre-kwh" v-model="form.estimated_kwh_100km" step="0.1" min="1" max="100" :placeholder="$t('common.example', { value: formatPerDistanceValue(16.5, 1) })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="vehicle-pre-rate" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.rateKwh', { currency: form.currency }) }}</label>
              <input id="vehicle-pre-rate" v-model.number="form.estimated_price_per_kwh" type="number" step="0.0001" min="0.01" max="5" :placeholder="$t('common.example', { value: $n(0.22) })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>
        </div>

        <!-- Tariffs & Home Charger Section -->
        <div v-if="form.powertrain !== 'ICE'" class="pt-2 border-t border-slate-800 space-y-3">
          <h4 class="text-xs font-bold text-amber-400 uppercase tracking-wider">{{ $t('tariffs.sectionTitle') }}</h4>

          <div>
            <label for="vehicle-tariff-plan" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('tariffs.planSelectLabel') }}</label>
            <select id="vehicle-tariff-plan" v-model="form.tariff_plan_id" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
              <option :value="null">{{ $t('tariffs.noPlanManual') }}</option>
              <option v-for="p in tariffPlans" :key="p.id" :value="p.id">
                {{ p.name }} ({{ p.plan_type }})
              </option>
            </select>
            <p class="text-[11px] text-slate-400 mt-1">{{ $t('tariffs.planSelectHint') }}</p>
          </div>

          <label class="flex items-start gap-2.5 p-3 rounded-xl bg-slate-800/40 border border-slate-700/60 cursor-pointer">
            <input type="checkbox" v-model="form.is_home_charger_default" class="mt-0.5 rounded border-slate-600 text-rose-600 focus:ring-rose-500 bg-slate-900" />
            <div class="text-xs">
              <span class="font-semibold text-white block">{{ $t('vehicles.homeChargerDefaultLabel') }}</span>
              <span class="text-slate-400 block mt-0.5">{{ $t('vehicles.homeChargerDefaultHint') }}</span>
            </div>
          </label>
        </div>

        <!-- TeslaMate API Section -->
        <div v-if="form.powertrain !== 'ICE'" class="pt-2 border-t border-slate-800 space-y-3">
          <h4 class="text-xs font-bold text-rose-400 uppercase tracking-wider">{{ $t('vehicles.vehicleFormModal.teslamateapiConnectionOptional') }}</h4>

          <div>
            <label for="vehicle-teslamate-api-url" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.teslamateapiBaseUrl') }}</label>
            <input id="vehicle-teslamate-api-url" v-model="form.teslamate_api_url" :placeholder="$t('vehicles.vehicleFormModal.eGHttp1921682')" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div>
            <label for="vehicle-teslamate-grafana-url" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.teslamateGrafanaUrlOptional') }}</label>
            <input id="vehicle-teslamate-grafana-url" v-model="form.teslamate_grafana_url" type="url" :placeholder="$t('vehicles.vehicleFormModal.eGHttp192168')" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            <p class="text-[11px] text-slate-500 mt-1">{{ $t('vehicles.vehicleFormModal.addsAnOpenInTeslamate') }}</p>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-teslamate-car-id" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.carIdInTeslamate') }}</label>
              <input id="vehicle-teslamate-car-id" v-model.number="form.teslamate_car_id" type="number" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="vehicle-teslamate-auth-type" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.authenticationMode') }}</label>
              <select id="vehicle-teslamate-auth-type" v-model="form.teslamate_auth_type" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white">
                <option value="NONE">{{ $t('vehicles.vehicleFormModal.noneLan') }}</option>
                <option value="BEARER">{{ $t('vehicles.vehicleFormModal.bearerTokenApiToken') }}</option>
                <option value="BASIC">{{ $t('vehicles.vehicleFormModal.httpBasicAuth') }}</option>
              </select>
            </div>
          </div>

          <div v-if="form.teslamate_auth_type === 'BEARER'">
            <label for="vehicle-teslamate-api-key" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.teslamateApiKeyToken') }}</label>
            <input id="vehicle-teslamate-api-key" v-model="form.teslamate_api_key" type="password" placeholder="••••••••" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
          </div>

          <div v-if="form.teslamate_auth_type === 'BASIC'" class="grid grid-cols-2 gap-3">
            <div>
              <label for="vehicle-teslamate-basic-user" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.basicAuthUser') }}</label>
              <input id="vehicle-teslamate-basic-user" v-model="form.teslamate_basic_user" placeholder="admin" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
            <div>
              <label for="vehicle-teslamate-basic-pass" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.basicAuthPassword') }}</label>
              <input id="vehicle-teslamate-basic-pass" v-model="form.teslamate_basic_pass" type="password" placeholder="••••••••" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white" />
            </div>
          </div>

          <!-- Test Connection inside modal -->
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
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="vehicle-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
