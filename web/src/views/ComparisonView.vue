<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import LoadError from '@/components/LoadError.vue'
import ComparisonEmptyState from '@/components/comparison/ComparisonEmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import { Scale as PageIcon } from 'lucide-vue-next'
import { t, te } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { distanceUnit, formatDistanceValue, fuelConsumptionUnit, volumeUnitLabel } from '@/units'
import { apiMessageText } from '@/services/apiError'
import { ref, reactive, computed, onMounted } from 'vue'
import { Plus, Trash2, Pencil, ArrowLeft, ArrowRight, Info, ChevronDown, GitCompare } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import { currencySymbol } from '@/currency'
import { CURRENCIES } from '@/utils/expenses'
import { comparisonCurrency, canCompareCurrencies } from '@/utils/comparisonCurrency'
import { canCharge } from '@/utils/vehicles'
import { scenarioSide } from '@/utils/comparisonSide'
import ComparisonCompare from '@/components/comparison/ComparisonCompare.vue'
import ComparisonResult from '@/components/comparison/ComparisonResult.vue'
import {
  applyComparisonDefaults,
  buildComparisonPayload,
  comparisonReferenceVehicle,
  comparisonStepErrorKey,
  emptyComparisonForm,
  type ComparisonForm,
  hasAdvancedOptions,
  scenarioToForm,
  MAX_COMPARE,
  toggleSelection,
} from '@/utils/comparisonForm'

const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()

const scenarios = ref<any[]>([])
const loading = ref(false)
const loadError = ref<string | null>(null)
const saving = ref(false)
const defaults = ref<any | null>(null)

// 'list' | 'edit' (steps 1-2) | 'result'
const view = ref<'list' | 'edit' | 'result' | 'compare'>('list')
const step = ref(1)
const editingId = ref<string | null>(null)
const currentScenario = ref<any | null>(null)
const resultSide = computed(() => scenarioSide(currentScenario.value, vehicleStore.vehicles))
const result = ref<any | null>(null)
const formError = ref('')

const fuelTypes = computed<any[]>(() => defaults.value?.ice || [])

// The server names each fuel in English; the catalog has the current language's name.
const fuelLabel = (f: { fuel_type: string; label: string }) => (te(`comparison.fuelTypes.${f.fuel_type}`) ? t(`comparison.fuelTypes.${f.fuel_type}`) : f.label)

// Electric and plug-in hybrid vehicles use their recorded energy costs.
const canCompareTrackedVehicle = computed(() => !!vehicleStore.activeVehicle && vehicleStore.canCharge)

const emptyForm = () => ({ ...emptyComparisonForm(canCompareTrackedVehicle.value), currency: vehicleStore.currency })
const form = reactive(emptyForm())
const showAdvanced = ref(false)
const formVehicle = computed(() => comparisonReferenceVehicle(form.vehicle_id, vehicleStore.activeVehicle, vehicleStore.vehicles))
const canCompareFormVehicle = computed(() => !!formVehicle.value && canCharge(formVehicle.value.powertrain))

const isRetro = computed(() => form.mode === 'RETROSPECTIVE')

const iceFields = [
  { key: 'purchase_price', label: 'comparison.fields.purchasePrice' },
  { key: 'resale_value', label: 'comparison.fields.resaleValue' },
  { key: 'maintenance_yearly', label: 'comparison.fields.maintenanceYearly' },
  { key: 'insurance_yearly', label: 'comparison.fields.insuranceYearly' },
  { key: 'tax_yearly', label: 'comparison.fields.taxYearly' },
] as const

const evFields = [
  { key: 'purchase_price', label: 'comparison.fields.purchasePriceNet' },
  { key: 'resale_value', label: 'comparison.fields.resaleValue' },
  { key: 'maintenance_yearly', label: 'comparison.fields.maintenanceYearly' },
  { key: 'insurance_yearly', label: 'comparison.fields.insuranceYearly' },
] as const

// A tracked scenario's amounts, defaults and labels follow its saved reference vehicle.
const currency = computed(() => {
  if (view.value === 'edit' && form.mode === 'PROJECTION') return /^[A-Z]{3}$/.test(form.currency.trim().toUpperCase()) ? form.currency.trim().toUpperCase() : vehicleStore.currency
  if (view.value === 'result') return comparisonCurrency(currentScenario.value, vehicleStore.vehicles) || vehicleStore.currency
  const referenceId = view.value === 'edit' ? form.vehicle_id : undefined
  return comparisonReferenceVehicle(referenceId, vehicleStore.activeVehicle, vehicleStore.vehicles)?.currency || vehicleStore.currency
})
const currencyOptions = computed(() => [...new Set([...CURRENCIES, ...vehicleStore.vehicles.map(v => v.currency), form.currency])].filter(code => /^[A-Z]{3}$/.test(code)))
const currencySign = computed(() => currencySymbol(currency.value))

function fmtKm(v: number): string {
  return formatDistanceValue(Number(v || 0))
}

async function loadScenarios() {
  loading.value = true
  loadError.value = null
  try {
    scenarios.value = await api.getComparisonScenarios()
  } catch (err) {
    console.error('Failed to load comparisons', err)
    loadError.value = (err as Error)?.message ?? ''
  } finally {
    loading.value = false
  }
}

async function loadDefaults() {
  // A tracked combustion vehicle prefills the ICE side of a projection with its measured consumption and fuel price
  const vehicleId = isRetro.value ? formVehicle.value?.id : vehicleStore.fuelOnly ? vehicleStore.activeVehicle?.id : undefined
  try {
    defaults.value = await api.getComparisonDefaults(vehicleId, currency.value)
  } catch (err) {
    console.error('Failed to load comparison defaults', err)
  }
}

function applyFuelDefaults() {
  const d = fuelTypes.value.find((f) => f.fuel_type === form.ice.fuel_type)
  if (!d) return
  form.ice.l_per_100km = d.l_per_100km
  form.ice.fuel_price = d.fuel_price
}

async function startNew(mode?: 'RETROSPECTIVE' | 'PROJECTION') {
  Object.assign(form, emptyForm())
  if (mode) form.mode = mode
  showAdvanced.value = false
  editingId.value = null
  formError.value = ''
  step.value = 1
  view.value = 'edit'
  await loadDefaults()
  applyDefaultsToForm()
}

function applyDefaultsToForm() {
  applyComparisonDefaults(form, defaults.value)
}

async function onModeChange() {
  await loadDefaults()
  applyDefaultsToForm()
}

function editScenario(sc: any) {
  Object.assign(form, scenarioToForm(sc, canCompareTrackedVehicle.value))
  showAdvanced.value = hasAdvancedOptions(form)
  editingId.value = sc.id
  formError.value = ''
  step.value = 1
  view.value = 'edit'
  loadDefaults()
}

// The adjusted scenario of the result page, saved over the open scenario or as another one.
const resultSaving = ref(false)
const resultSaveError = ref('')

async function saveAdjusted(adjusted: ComparisonForm, asNew: boolean) {
  const id = currentScenario.value?.id
  if (resultSaving.value || (!asNew && !id)) return
  resultSaving.value = true
  resultSaveError.value = ''
  try {
    const payload = buildComparisonPayload(adjusted, vehicleStore.activeVehicle?.id)
    const saved = asNew ? await api.createComparisonScenario(payload) : await api.updateComparisonScenario(id, payload)
    await loadScenarios()
    await openResult(saved)
  } catch (err: any) {
    resultSaveError.value = err?.message || t('comparison.errors.saveFailed')
  } finally {
    resultSaving.value = false
  }
}

async function nextStep() {
  const errorKey = comparisonStepErrorKey(step.value, form, canCompareFormVehicle.value)
  formError.value = errorKey ? t(errorKey) : ''
  if (formError.value) return
  if (step.value < 2) {
    step.value += 1
    return
  }
  await saveAndCompute()
}

function prevStep() {
  formError.value = ''
  if (step.value > 1) step.value -= 1
  else view.value = 'list'
}

async function saveAndCompute() {
  saving.value = true
  try {
    const payload = buildComparisonPayload(form, vehicleStore.activeVehicle?.id)
    const saved = editingId.value
      ? await api.updateComparisonScenario(editingId.value, payload)
      : await api.createComparisonScenario(payload)
    await loadScenarios()
    await openResult(saved)
  } catch (err: any) {
    formError.value = err?.message || t('comparison.errors.saveFailed')
  } finally {
    saving.value = false
  }
}

async function openResult(sc: any) {
  if (!comparisonCurrency(sc, vehicleStore.vehicles)) {
    editScenario(sc)
    formError.value = t('comparison.currency.legacy')
    return
  }
  currentScenario.value = sc
  result.value = null
  view.value = 'result'
  try {
    result.value = await api.getComparisonResult(sc.id)
  } catch (err: any) {
    await showAlert(err?.message || t('comparison.errors.calcFailed'), t('comparison.comparisonView.title'), 'danger')
    view.value = 'list'
    return
  }
}

async function removeScenario(sc: any) {
  const ok = await showConfirm({
    title: t('comparison.comparisonView.deleteTitle'),
    message: t('comparison.comparisonView.deleteMessage', { name: sc.name }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteComparisonScenario(sc.id)
    await loadScenarios()
  } catch (err: any) {
    await showAlert(err?.message || t('comparison.errors.deleteFailed'), t('comparison.comparisonView.deletion'), 'danger')
  }
}

// --- Compare several scenarios ---

const selectedIds = ref<string[]>([])
const compareItems = ref<{ scenario: any; result: any }[]>([])
function toggleSelected(id: string) {
  selectedIds.value = toggleSelection(selectedIds.value, id)
}

async function openCompare() {
  const chosen = scenarios.value.filter((s) => selectedIds.value.includes(s.id))
  if (!canCompareCurrencies(chosen, vehicleStore.vehicles)) {
    await showAlert(t('comparison.currency.same'), t('comparison.comparisonView.title'), 'warning')
    return
  }
  try {
    const results = await Promise.all(chosen.map((s) => api.getComparisonResult(s.id)))
    compareItems.value = chosen.map((scenario, i) => ({ scenario, result: results[i] }))
    view.value = 'compare'
  } catch (err: any) {
    await showAlert(err?.message || t('comparison.errors.compareFailed'), t('comparison.comparisonView.title'), 'danger')
  }
}

onMounted(async () => {
  await loadScenarios()
})
</script>

<template>
  <div class="space-y-6">
    <PageHeader :title="$t('comparison.comparisonView.electricCombustionComparison')" :icon="PageIcon">
      {{ $t('comparison.comparisonView.forInformationOnlyNoneOf') }}
      <template #actions>
      <button
        v-if="view === 'list'"
        class="btn btn-lg btn-primary"
        @click="startNew()"
      >
        <Plus class="w-4 h-4" /> {{ $t('comparison.comparisonView.newComparison') }}
      </button>
    
      </template>
    </PageHeader>

    <!-- Scenario list -->
    <div v-if="view === 'list'">
      <div v-if="loading" class="text-sm text-slate-400">{{ $t('comparison.comparisonView.loading') }}</div>
      <LoadError v-else-if="loadError !== null" :message="loadError" @retry="loadScenarios" />
      <ComparisonEmptyState v-else-if="scenarios.length === 0" @start="startNew" />
      <div v-else class="space-y-3">
      <div v-if="scenarios.length >= 2" class="flex items-center justify-between gap-3 text-xs text-slate-400">
        <span>{{ $t('comparison.comparisonView.tick2Or3Comparisons') }}</span>
        <button
          type="button"
          :disabled="selectedIds.length < 2"
          class="bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 border border-slate-700 font-semibold px-3 py-2 rounded-xl flex items-center gap-2"
          @click="openCompare"
        >
          <GitCompare class="w-4 h-4" /> {{ $t('comparison.comparisonView.compare', { length: selectedIds.length }) }}
        </button>
      </div>
      <ul class="grid gap-3 md:grid-cols-2">
        <li v-for="sc in scenarios" :key="sc.id" class="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex items-center justify-between gap-3">
          <input
            :id="`cmp-select-${sc.id}`"
            type="checkbox"
            class="rounded border-slate-600 bg-slate-800 shrink-0"
            :checked="selectedIds.includes(sc.id)"
            :disabled="!selectedIds.includes(sc.id) && selectedIds.length >= MAX_COMPARE"
            :aria-label="$t('comparison.comparisonView.selectFor', { name: sc.name })"
            @change="toggleSelected(sc.id)"
          />
          <button class="text-left min-w-0 flex-1" @click="openResult(sc)">
            <div class="text-sm font-semibold text-white truncate">{{ sc.name }}</div>
            <div class="text-xs text-slate-400">
              {{ sc.mode === 'RETROSPECTIVE' ? $t('comparison.comparisonView.trackedVehicle') : $t('comparison.comparisonView.projection') }} · {{ $t('comparison.comparisonView.scenarioUsage', { unit: distanceUnit(), km: fmtKm(sc.annual_km), years: sc.years }) }}
            </div>
          </button>
          <div class="flex items-center gap-1.5 shrink-0">
            <button :aria-label="$t('comparison.comparisonView.editScenario', { name: sc.name })" class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300" @click="editScenario(sc)">
              <Pencil class="w-4 h-4" />
            </button>
            <button :aria-label="$t('comparison.comparisonView.deleteScenario', { name: sc.name })" class="p-2 rounded-lg bg-slate-800 hover:bg-red-900/60 text-red-300" @click="removeScenario(sc)">
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </li>
      </ul>
      </div>
    </div>

    <!-- Compare several scenarios -->
    <div v-else-if="view === 'compare'" class="space-y-5">
      <button class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5 no-print" @click="view = 'list'">
        <ArrowLeft class="w-4 h-4" /> {{ $t('comparison.comparisonView.myComparisons') }}
      </button>
      <ComparisonCompare :items="compareItems" />
    </div>

    <!-- Editor (steps 1 and 2) -->
    <form v-else-if="view === 'edit'" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 space-y-5" @submit.prevent="nextStep">
      <div class="text-xs text-slate-400">{{ $t('comparison.comparisonView.stepOf', { step, name: step === 1 ? $t('comparison.comparisonView.usage') : isRetro ? $t('comparison.comparisonView.equivalentIce') : $t('comparison.comparisonView.comparedVehicles') }) }}</div>

      <template v-if="step === 1">
        <div>
          <label for="cmp-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.comparisonName') }}</label>
          <input id="cmp-name" v-model="form.name" maxlength="100" :placeholder="$t('comparison.comparisonView.eGComparedWithA')" class="field" />
        </div>
        <div>
          <label for="cmp-mode" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.comparisonType') }}</label>
          <select id="cmp-mode" v-model="form.mode" :disabled="!!editingId" class="field" @change="onModeChange">
            <option value="RETROSPECTIVE" :disabled="!canCompareFormVehicle">
              {{ $t('comparison.comparisonView.myTrackedEv') }}{{ formVehicle ? ` (${formVehicle.name})` : '' }} {{ $t('comparison.comparisonView.vsIce') }}
            </option>
            <option value="PROJECTION">{{ $t('comparison.comparisonView.projectionICompareTwoVehicles') }}</option>
          </select>
          <p v-if="isRetro" class="text-xs text-slate-400 mt-1">{{ $t('comparison.comparisonView.theElectricVehicleSCosts') }}</p>
        </div>
        <div v-if="!isRetro">
          <label for="cmp-currency" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.currency.label') }}</label>
          <select id="cmp-currency" v-model="form.currency" required class="field" @change="loadDefaults">
            <option disabled value="">{{ $t('comparison.errors.currency') }}</option>
            <option v-for="code in currencyOptions" :key="code" :value="code">{{ code }}</option>
          </select>
          <p class="text-xs text-slate-400 mt-1">{{ $t('comparison.currency.hint') }}</p>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="cmp-km" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.kilometresPerYear', { unit: distanceUnit() }) }}</label>
            <DistanceInput id="cmp-km" v-model="form.annual_km" :digits="0" min="1" step="any" class="field" />
            <p v-if="defaults && isRetro" class="text-xs text-slate-400 mt-1">
              {{ defaults.annual_km_from_data ? $t('comparison.comparisonView.fromHistory') : $t('comparison.comparisonView.defaultValue') }}
            </p>
          </div>
          <div>
            <label for="cmp-years" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.durationYears') }}</label>
            <input id="cmp-years" v-model.number="form.years" type="number" min="1" max="15" class="field" />
          </div>
        </div>
      </template>

      <template v-else>
        <div class="space-y-3">
          <h2 class="text-sm font-semibold text-warning-300">{{ $t('comparison.comparisonView.equivalentCombustionVehicle') }}</h2>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="cmp-fuel" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.fuel') }}</label>
              <select id="cmp-fuel" v-model="form.ice.fuel_type" class="field" @change="applyFuelDefaults">
                <option v-for="f in fuelTypes" :key="f.fuel_type" :value="f.fuel_type">{{ fuelLabel(f) }}</option>
              </select>
            </div>
            <div>
              <label for="cmp-ice-l100" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.consumptionL100Km', { unit: fuelConsumptionUnit() }) }}</label>
              <DistanceInput kind="consumption" :digits="2" id="cmp-ice-l100" v-model="form.ice.l_per_100km" step="any" class="field" />
            </div>
            <div>
              <label for="cmp-ice-price" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.fuelPriceL', { cur: currencySign, unit: volumeUnitLabel() }) }}</label>
              <DistanceInput kind="per-volume" :digits="3" id="cmp-ice-price" v-model="form.ice.fuel_price" min="0" class="field" />
            </div>
          </div>
          <p class="text-xs text-slate-400 flex items-center gap-1"><Info class="w-3 h-3" /> {{ defaults?.source ? apiMessageText(defaults.source) : $t('comparison.comparisonView.indicative') }}</p>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div v-for="f in iceFields" :key="f.key">
              <label :for="`cmp-ice-${f.key}`" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t(f.label, { cur: currencySign }) }}</label>
              <NumberInput :id="`cmp-ice-${f.key}`" v-model="form.ice[f.key]" min="0" class="field" />
            </div>
          </div>
        </div>

        <div v-if="!isRetro" class="space-y-3 pt-2 border-t border-slate-800">
          <h2 class="text-sm font-semibold text-info-300">{{ $t('comparison.comparisonView.electricVehicle') }}</h2>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="cmp-ev-kwh" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.consumptionKwh100Km', { unit: distanceUnit() }) }}</label>
              <DistanceInput kind="per-distance" id="cmp-ev-kwh" v-model="form.tracked.kwh_per_100km" min="0.1" step="any" class="field" />
            </div>
            <div>
              <label for="cmp-ev-price" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.averageElectricityPriceKwh', { cur: currencySign }) }}</label>
              <NumberInput id="cmp-ev-price" v-model="form.tracked.eur_per_kwh" min="0" class="field" />
            </div>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div v-for="f in evFields" :key="f.key">
              <label :for="`cmp-ev-${f.key}`" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t(f.label, { cur: currencySign }) }}</label>
              <NumberInput :id="`cmp-ev-${f.key}`" v-model="form.tracked[f.key]" min="0" class="field" />
            </div>
          </div>
        </div>
      </template>

      <div v-if="step === 2" class="pt-2 border-t border-slate-800">
        <button
          type="button"
          class="flex items-center gap-1.5 text-xs font-semibold text-slate-300 hover:text-white"
          :aria-expanded="showAdvanced"
          aria-controls="cmp-advanced"
          @click="showAdvanced = !showAdvanced"
        >
          <ChevronDown class="w-4 h-4 transition-transform" :class="showAdvanced ? 'rotate-180' : ''" /> {{ $t('comparison.comparisonView.fineTuneInflationGrants') }}
        </button>
        <div v-show="showAdvanced" id="cmp-advanced" class="mt-3 space-y-3">
          <p class="text-xs text-slate-400">{{ $t('comparison.comparisonView.averageYearlyPriceChangeApplied') }}</p>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="cmp-infl-fuel" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.fuelYear') }}</label>
              <NumberInput id="cmp-infl-fuel" v-model="form.options.fuel_inflation_pct" min="-10" max="30" class="field" />
            </div>
            <div>
              <label for="cmp-infl-elec" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.electricityYear') }}</label>
              <NumberInput id="cmp-infl-elec" v-model="form.options.electricity_inflation_pct" min="-10" max="30" class="field" />
            </div>
            <div>
              <label for="cmp-infl-cost" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.maintenanceInsuranceTaxesYear') }}</label>
              <NumberInput id="cmp-infl-cost" v-model="form.options.cost_inflation_pct" min="-10" max="30" class="field" />
            </div>
          </div>
          <div v-if="!isRetro">
            <label for="cmp-ev-incentives" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.electricPurchaseGrantsDeductedFrom', { cur: currencySign }) }}</label>
            <NumberInput id="cmp-ev-incentives" v-model="form.options.tracked_incentives" min="0" class="field sm:w-1/2" />
          </div>
        </div>
      </div>

      <p v-if="formError" class="text-xs text-red-300" role="alert">{{ formError }}</p>

      <div class="flex items-center justify-between pt-1">
        <button type="button" class="btn btn-lg btn-secondary" @click="prevStep">
          <ArrowLeft class="w-4 h-4" /> {{ step === 1 ? $t('common.cancel') : $t('comparison.comparisonView.back') }}
        </button>
        <button type="submit" :disabled="saving" class="btn btn-lg btn-primary">
          {{ step === 2 ? (saving ? $t('comparison.comparisonView.calculating') : $t('comparison.comparisonView.viewResult')) : $t('comparison.comparisonView.next') }} <ArrowRight class="w-4 h-4" />
        </button>
      </div>
    </form>

    <!-- Result -->
    <ComparisonResult
      v-else-if="view === 'result'"
      :result="result"
      :scenario="currentScenario"
      :side="resultSide"
      :currency="currency"
      :reference-vehicle-id="vehicleStore.activeVehicle?.id"
      @back="view = 'list'"
      @edit="editScenario(currentScenario)"
      :saving="resultSaving"
      :save-error="resultSaveError"
      @save="saveAdjusted($event, false)"
      @save-as="saveAdjusted($event, true)"
    />
  </div>
</template>
