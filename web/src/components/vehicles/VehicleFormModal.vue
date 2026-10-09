<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import NumberInput from '@/components/NumberInput.vue'
import { intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import {
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  Link2,
  ChevronDown,
  ChevronRight,
  Zap,
  Flame,
  Copy,
  Sliders,
} from 'lucide-vue-next'
import { canLinkTeslaMate, emptyVehicleForm, vehicleFormFrom } from '@/utils/vehicles'
import { CURRENCIES } from '@/utils/expenses'
import { distanceUnit, formatDistanceValue, formatPerDistanceValue } from '@/units'
import { useVehicleStore } from '@/stores/vehicle'
import { useSubmit } from '@/composables/useSubmit'

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

const powertrainChoices = ['EV', 'PHEV', 'REEV', 'ICE'] as const

function setPowertrain(p: (typeof powertrainChoices)[number]) {
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

const { pending: submitting, run: runOnce } = useSubmit()

async function handleSaveAction() {
  try {
    const payload: Record<string, any> = { ...form.value }
    if (!canLinkTeslaMate(payload.powertrain)) {
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
const handleSave = () => runOnce(handleSaveAction)

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
  <ModalShell
    v-model:open="open"
    :title="isEditing ? $t('vehicles.vehicleFormModal.edit') : $t('shell.topBar.addAVehicle')"
  >
    <form id="vehicle-modal-form" @submit.prevent="handleSave" class="space-y-4">
      <!-- 1. Identité du véhicule : Marque & Modèle (100% Free text according to CLAUDE.md) -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label for="vehicle-make" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('onboarding.onboardingView.make') }}</label>
          <input
            id="vehicle-make"
            v-model="form.make"
            :placeholder="$t('onboarding.onboardingView.makePlaceholder')"
            class="field transition-colors"
          />
        </div>
        <div>
          <label for="vehicle-model" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('onboarding.onboardingView.model') }}</label>
          <input
            id="vehicle-model"
            v-model="form.model"
            :placeholder="$t('onboarding.onboardingView.modelPlaceholder')"
            class="field transition-colors"
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
          class="field transition-colors"
        />
      </div>

      <!-- Motorisation (Boutons visuels au lieu d'un simple select) -->
      <div>
        <span class="block text-xs font-semibold text-slate-300 mb-1.5">{{ $t('vehicles.vehicleFormModal.powertrain') }}</span>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <button
            v-for="p in powertrainChoices"
            :key="p"
            type="button"
            :disabled="isEditing"
            @click="setPowertrain(p)"
            class="p-2.5 rounded-xl border flex items-center gap-2.5 transition-all text-left disabled:opacity-50 disabled:cursor-not-allowed"
            :class="form.powertrain === p ? 'bg-rose-500/10 border-rose-500/50 text-white shadow-sm ring-1 ring-rose-500/20' : 'bg-slate-800/40 border-slate-700/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'"
          >
            <Flame v-if="p === 'ICE'" class="w-4 h-4 text-orange-400 shrink-0" />
            <Zap v-else class="w-4 h-4 text-warning-400 shrink-0" />
            <span class="text-xs font-semibold leading-tight">{{ $t(`vehicles.powertrainOptions.${p}`) }}</span>
          </button>
        </div>
      </div>

      <!-- Devise & Kilométrage -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label for="vehicle-current-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.currentMileage', { unit: distanceUnit() }) }}</label>
          <DistanceInput
            id="vehicle-current-odometer"
            v-model="form.current_odometer"
            step="1"
            :disabled="!!form.teslamate_api_url && connectTeslaMate"
            class="field"
          />
          <p class="mt-1 text-xs text-slate-400">{{ $t('vehicles.vehicleFormModal.currentMileageHelp') }}</p>
        </div>
        <div>
          <label for="vehicle-currency" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.currency') }}</label>
          <select
            id="vehicle-currency"
            v-model="form.currency"
            :disabled="isEditing"
            class="field"
          >
            <option v-for="c in CURRENCIES" :key="c" :value="c">{{ c }}</option>
          </select>
          <p v-if="isEditing" class="mt-1 text-xs text-slate-400">{{ $t('vehicles.vehicleFormModal.currencyFixed') }}</p>
        </div>
      </div>

      <!-- VIN Optionnel -->
      <div>
        <label for="vehicle-vin" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.vinOptional') }}</label>
        <input
          id="vehicle-vin"
          v-model="form.vin"
          placeholder="VIN"
          class="field transition-colors"
        />
      </div>

      <!-- 2. TeslaMate synchronization (electric vehicles); the tracking mode follows from it on the server -->
      <div v-if="canLinkTeslaMate(form.powertrain)" class="pt-3 border-t border-slate-800">
        <label
          for="vehicle-connect-teslamate"
          class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-all"
          :class="connectTeslaMate
            ? 'bg-rose-500/10 border-rose-500/50 shadow-sm ring-1 ring-rose-500/20'
            : 'bg-slate-800/40 border-slate-700/60 hover:bg-slate-800'"
        >
          <input id="vehicle-connect-teslamate" v-model="connectTeslaMate" type="checkbox" class="select-box mt-0.5" />
          <div>
            <span class="text-xs font-semibold text-white block">{{ $t('vehicles.vehicleFormModal.teslamateSync') }}</span>
            <span class="text-xs text-slate-400 block mt-0.5">{{ $t('vehicles.vehicleFormModal.teslamateSyncDesc') }}</span>
          </div>
        </label>
      </div>

      <!-- 3. Paramètres de télémétrie connectée (Visible UNIQUEMENT si EV et CONNECTED) -->
      <div v-if="canLinkTeslaMate(form.powertrain) && connectTeslaMate" class="pt-3 border-t border-slate-800 space-y-3">
        <div class="flex items-center justify-between">
          <h4 class="text-xs font-bold text-rose-400 uppercase tracking-wider">{{ $t('vehicles.vehicleFormModal.telemetrySettings') }}</h4>
          <span class="text-xs px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-400 border border-rose-500/20">TeslaMate</span>
        </div>

        <!-- Bouton de réutilisation pratique depuis un autre véhicule connecté -->
        <div v-if="existingConnectedVehicles.length > 0 && !isEditing" class="p-2.5 rounded-xl bg-slate-800/60 border border-slate-700/60 flex flex-wrap items-center gap-2 text-xs">
          <span class="text-slate-400 text-xs">{{ $t('vehicles.vehicleFormModal.copyFromExisting', { name: '' }) }}:</span>
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
            class="field"
          />
        </div>

        <div>
          <label for="vehicle-teslamate-grafana-url" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.teslamateGrafanaUrlOptional') }}</label>
          <input
            id="vehicle-teslamate-grafana-url"
            v-model="form.teslamate_grafana_url"
            type="url"
            :placeholder="$t('vehicles.vehicleFormModal.eGHttp192168')"
            class="field"
          />
          <p class="text-xs text-slate-400 mt-1">{{ $t('vehicles.vehicleFormModal.addsAnOpenInTeslamate') }}</p>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="vehicle-teslamate-car-id" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.carIdInTeslamate') }}</label>
            <input
              id="vehicle-teslamate-car-id"
              v-model.number="form.teslamate_car_id"
              type="number"
              min="1"
              class="field"
            />
          </div>
          <div>
            <label for="vehicle-teslamate-auth-type" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.authenticationMode') }}</label>
            <select
              id="vehicle-teslamate-auth-type"
              v-model="form.teslamate_auth_type"
              class="field"
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
            class="field"
          />
        </div>

        <div v-if="form.teslamate_auth_type === 'BASIC'" class="grid grid-cols-2 gap-3">
          <div>
            <label for="vehicle-teslamate-basic-user" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.basicAuthUser') }}</label>
            <input
              id="vehicle-teslamate-basic-user"
              v-model="form.teslamate_basic_user"
              placeholder="admin"
              class="field"
            />
          </div>
          <div>
            <label for="vehicle-teslamate-basic-pass" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.basicAuthPassword') }}</label>
            <input
              id="vehicle-teslamate-basic-pass"
              v-model="form.teslamate_basic_pass"
              type="password"
              placeholder="••••••••"
              class="field"
            />
          </div>
        </div>

        <!-- Test de connexion -->
        <div v-if="form.teslamate_api_url" class="pt-2">
          <button
            type="button"
            @click="testModalConnection"
            :disabled="modalTestLoading"
            class="btn btn-secondary w-full"
          >
            <RefreshCw v-if="modalTestLoading" class="w-3.5 h-3.5 animate-spin text-rose-400" />
            <Link2 v-else class="w-3.5 h-3.5 text-rose-400" />
            <span>{{ modalTestLoading ? $t('vehicles.vehicleFormModal.testing') : $t('vehicles.vehicleFormModal.testConnection') }}</span>
          </button>

          <div
            v-if="modalTestResult"
            class="mt-2.5 p-3 rounded-xl text-xs flex items-start gap-2"
            :class="modalTestResult.success ? 'bg-success-500/10 text-success-300 border border-success-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20'"
          >
            <CheckCircle2 v-if="modalTestResult.success" class="w-4 h-4 shrink-0 text-success-400 mt-0.5" />
            <AlertCircle v-else class="w-4 h-4 shrink-0 text-danger-400 mt-0.5" />
            <div class="flex-1">
              <div v-if="modalTestResult.success">
                <strong class="font-semibold">{{ $t('vehicles.vehicleFormModal.connectionSuccessful') }}</strong>
                <p class="text-xs text-success-200/80 mt-0.5">
                  {{ $t('vehicles.vehicleFormModal.testStatus', { unit: distanceUnit(), state: modalTestResult.status?.state || $t('vehicles.vehicleCard.online'), odometer: formatDistanceValue(modalTestResult.status?.odometer || 0) }) }}
                </p>
              </div>
              <div v-else>
                <strong class="font-semibold">{{ $t('vehicles.vehicleFormModal.connectionFailed') }}</strong>
                <p class="text-xs text-danger-200/90 mt-0.5">{{ modalTestResult.error }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 4. Options complémentaires (Repliables pour ne pas encombrer la création) -->
      <div v-if="canLinkTeslaMate(form.powertrain)" class="pt-3 border-t border-slate-800">
        <button
          type="button"
          @click="showAdvanced = !showAdvanced"
          class="w-full flex items-center justify-between py-2 text-xs font-semibold text-slate-300 hover:text-white transition-colors"
        >
          <span class="flex items-center gap-2">
            <Sliders class="w-4 h-4 text-warning-400" />
            {{ $t('vehicles.vehicleFormModal.advancedOptions') }}
          </span>
          <ChevronDown v-if="showAdvanced" class="w-4 h-4 text-slate-400" />
          <ChevronRight v-else class="w-4 h-4 text-slate-400" />
        </button>
        <p class="text-xs text-slate-400 mb-2">{{ $t('vehicles.vehicleFormModal.advancedOptionsDesc') }}</p>

        <div v-if="showAdvanced" class="space-y-3.5 pt-2">
          <!-- Tarifs & Borne -->
          <div>
            <label for="vehicle-tariff-plan" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('tariffs.planSelectLabel') }}</label>
            <select id="vehicle-tariff-plan" v-model="form.tariff_plan_id" class="field">
              <option :value="null">{{ $t('tariffs.noPlanManual') }}</option>
              <option v-for="p in tariffPlans.filter((plan) => plan.currency === form.currency)" :key="p.id" :value="p.id">
                {{ p.name }} ({{ p.plan_type }})
              </option>
            </select>
            <p class="text-xs text-slate-400 mt-1">
              {{ $t('tariffs.planSelectHint') }}
              <router-link to="/account" class="text-primary-300 underline" @click="open = false">{{ $t('tariffs.editor.manageLink') }}</router-link>
            </p>
          </div>

          <label for="vehicle-home-charger-default" class="flex items-start gap-2.5 p-3 rounded-xl bg-slate-800/40 border border-slate-700/60 cursor-pointer">
            <input id="vehicle-home-charger-default" type="checkbox" v-model="form.is_home_charger_default" class="select-box mt-0.5" />
            <div class="text-xs">
              <span class="font-semibold text-white block">{{ $t('vehicles.homeChargerDefaultLabel') }}</span>
              <span class="text-slate-400 block mt-0.5">{{ $t('vehicles.homeChargerDefaultHint') }}</span>
            </div>
          </label>

          <!-- Consommation et tarif estimé -->
          <div class="grid grid-cols-2 gap-3 pt-1">
            <div>
              <label for="vehicle-pre-kwh" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.consumptionKwh100km', { unit: distanceUnit() }) }}</label>
              <DistanceInput kind="per-distance" id="vehicle-pre-kwh" v-model="form.estimated_kwh_100km" step="0.1" min="1" max="100" :placeholder="$t('common.example', { value: formatPerDistanceValue(16.5, 1) })" class="field" />
            </div>
            <div>
              <label for="vehicle-pre-rate" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.vehicleFormModal.rateKwh', { currency: form.currency }) }}</label>
              <NumberInput id="vehicle-pre-rate" v-model="form.estimated_price_per_kwh" min="0.01" max="5" :placeholder="$t('common.example', { value: $n(0.22) })" class="field" />
            </div>
          </div>
        </div>
      </div>
    </form>
    <template #footer>
      <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
        {{ $t('common.cancel') }}
      </button>
      <button type="submit" :disabled="submitting" form="vehicle-modal-form" class="btn btn-lg btn-primary">
        {{ $t('common.save') }}
      </button>
    </template>
  </ModalShell>
</template>
