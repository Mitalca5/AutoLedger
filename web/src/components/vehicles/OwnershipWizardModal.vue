<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { currencySymbol, formatAmount } from '@/currency'
import { useConfirm } from '@/composables/useConfirm'
import { RefreshCw, X, FileText, ChevronLeft, ChevronRight, Check, Wallet, CreditCard, KeyRound } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import {
  ownershipSteps,
  isLeaseType,
  isOwnedPhase as isOwnedPhaseType,
  isPurchaseType,
  leasePreview as buildLeasePreview,
  loanPreview as buildLoanPreview,
  ownershipFormFrom,
  ownershipPayload,
  ownershipStepError,
} from '@/utils/vehicles'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistance, formatDistanceValue, formatPerDistanceValue } from '@/units'

// The acquisition contract of a vehicle (purchase, loan, LOA, LLD) in three steps. \`ownership\` is the saved
// contract (null when there is none); saving reports the stored contract, deleting reports deleted.
const props = defineProps<{ vehicle: any | null; ownership: any | null }>()
const emit = defineEmits<{ saved: [ownership: any]; deleted: [] }>()
const currency = computed(() => props.vehicle?.currency || 'EUR')
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showConfirm, showAlert } = useConfirm()

const ownershipVehicle = computed(() => props.vehicle)
const currentOwnershipStep = ref(1)
const ownershipForm = ref(ownershipFormFrom(null, {}))

watch(open, (isOpen) => {
  if (!isOpen || !props.vehicle) return
  currentOwnershipStep.value = 1
  ownershipForm.value = ownershipFormFrom(props.ownership, props.vehicle)
})

const isLease = computed(() => isLeaseType(ownershipForm.value))
const isPurchase = computed(() => isPurchaseType(ownershipForm.value))
const isOwnedPhase = computed(() => isOwnedPhaseType(ownershipForm.value))
const loanPreview = computed(() => buildLoanPreview(ownershipForm.value))
const leasePreview = computed(() => buildLeasePreview(ownershipForm.value))

function validateOwnershipStep(step: number): boolean {
  const error = ownershipStepError(ownershipForm.value, step)
  if (error) {
    showAlert(error, t('common.requiredField'), 'warning')
    return false
  }
  return true
}

function nextOwnershipStep() {
  if (validateOwnershipStep(currentOwnershipStep.value)) {
    if (currentOwnershipStep.value < 3) {
      currentOwnershipStep.value++
    }
  }
}

function prevOwnershipStep() {
  if (currentOwnershipStep.value > 1) {
    currentOwnershipStep.value--
  }
}

async function handleSaveOwnership() {
  if (!props.vehicle) return
  if (!validateOwnershipStep(1) || !validateOwnershipStep(2)) return
  try {
    const saved = await api.saveOwnership(props.vehicle.id, ownershipPayload(ownershipForm.value))
    open.value = false
    emit('saved', saved)
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

async function handleDeleteOwnership() {
  if (!props.vehicle) return
  const ok = await showConfirm({
    title: t('vehicles.ownershipWizardModal.deleteTitle'),
    message: t('vehicles.ownershipWizardModal.deleteMessage'),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteOwnership(props.vehicle.id)
    open.value = false
    emit('deleted')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <div
    v-if="open && ownershipVehicle"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Modal Header -->
      <div class="px-5 py-4 border-b border-slate-800/80 shrink-0 bg-slate-900/95">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white flex items-center gap-2 truncate pr-2">
            <FileText class="w-5 h-5 text-indigo-400 shrink-0" />
            <span class="truncate">{{ $t('vehicles.ownershipWizardModal.acquisitionAndFinancing', { name: ownershipVehicle.name }) }}</span>
          </h3>
          <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors shrink-0">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Wizard Stepper Indicator -->
        <div class="grid grid-cols-3 gap-2 mt-4 pt-3 border-t border-slate-800/60">
          <button
            v-for="s in ownershipSteps()"
            :key="s.step"
            type="button"
            @click="s.step < currentOwnershipStep ? currentOwnershipStep = s.step : null"
            :disabled="s.step > currentOwnershipStep"
            class="flex items-center gap-2 p-1.5 rounded-xl text-left transition-colors"
            :class="[
              currentOwnershipStep === s.step
                ? 'bg-indigo-500/15 border border-indigo-500/40 text-indigo-300'
                : currentOwnershipStep > s.step
                ? 'bg-slate-800/60 text-slate-300 hover:bg-slate-800 cursor-pointer'
                : 'bg-slate-900/40 text-slate-400 opacity-60 cursor-not-allowed'
            ]"
          >
            <div
              class="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold shrink-0 transition-colors"
              :class="[
                currentOwnershipStep === s.step
                  ? 'bg-indigo-500 text-white shadow-sm shadow-indigo-500/40'
                  : currentOwnershipStep > s.step
                  ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                  : 'bg-slate-800 text-slate-400 border border-slate-700'
              ]"
            >
              <Check v-if="currentOwnershipStep > s.step" class="w-3.5 h-3.5" />
              <span v-else>{{ s.step }}</span>
            </div>
            <div class="min-w-0 hidden sm:block">
              <div class="text-xs font-semibold truncate">{{ s.title }}</div>
              <div class="text-xs text-slate-400 truncate">{{ s.description }}</div>
            </div>
          </button>
        </div>
      </div>

      <!-- Wizard Content -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <!-- STEP 1: Acquisition Type & Dates -->
        <div v-show="currentOwnershipStep === 1" class="space-y-4">
          <div>
            <span class="block text-xs font-semibold text-slate-300 mb-2">{{ $t('vehicles.ownershipWizardModal.acquisitionMode') }}</span>
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5">
              <button
                type="button"
                @click="ownershipForm.acquisition_type = 'CASH'"
                class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                :class="[
                  ownershipForm.acquisition_type === 'CASH'
                    ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                    : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                ]"
              >
                <div class="flex items-center justify-between mb-2">
                  <Wallet class="w-5 h-5 text-emerald-400" />
                  <span v-if="ownershipForm.acquisition_type === 'CASH'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                </div>
                <div>
                  <div class="text-xs font-bold text-white">{{ $t('vehicles.ownershipWizardModal.cash') }}</div>
                  <div class="text-xs text-slate-400">{{ $t('vehicles.ownershipWizardModal.directPurchase') }}</div>
                </div>
              </button>

              <button
                type="button"
                @click="ownershipForm.acquisition_type = 'LOAN'"
                class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                :class="[
                  ownershipForm.acquisition_type === 'LOAN'
                    ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                    : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                ]"
              >
                <div class="flex items-center justify-between mb-2">
                  <CreditCard class="w-5 h-5 text-indigo-400" />
                  <span v-if="ownershipForm.acquisition_type === 'LOAN'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                </div>
                <div>
                  <div class="text-xs font-bold text-white">{{ $t('vehicles.ownershipWizardModal.loan') }}</div>
                  <div class="text-xs text-slate-400">{{ $t('vehicles.ownershipWizardModal.bankLoan') }}</div>
                </div>
              </button>

              <button
                type="button"
                @click="ownershipForm.acquisition_type = 'LOA'"
                class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                :class="[
                  ownershipForm.acquisition_type === 'LOA'
                    ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                    : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                ]"
              >
                <div class="flex items-center justify-between mb-2">
                  <KeyRound class="w-5 h-5 text-amber-400" />
                  <span v-if="ownershipForm.acquisition_type === 'LOA'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                </div>
                <div>
                  <div class="text-xs font-bold text-white">{{ $t('vehicles.ownershipWizardModal.loa') }}</div>
                  <div class="text-xs text-slate-400">{{ $t('vehicles.ownershipWizardModal.purchaseOption') }}</div>
                </div>
              </button>

              <button
                type="button"
                @click="ownershipForm.acquisition_type = 'LLD'"
                class="p-3 rounded-xl border text-left flex flex-col justify-between transition-all"
                :class="[
                  ownershipForm.acquisition_type === 'LLD'
                    ? 'border-indigo-500 bg-indigo-500/15 text-white shadow-sm'
                    : 'border-slate-800 bg-slate-800/60 text-slate-300 hover:border-slate-700 hover:bg-slate-800'
                ]"
              >
                <div class="flex items-center justify-between mb-2">
                  <RefreshCw class="w-5 h-5 text-sky-400" />
                  <span v-if="ownershipForm.acquisition_type === 'LLD'" class="w-2 h-2 rounded-full bg-indigo-400"></span>
                </div>
                <div>
                  <div class="text-xs font-bold text-white">{{ $t('vehicles.ownershipWizardModal.lld') }}</div>
                  <div class="text-xs text-slate-400">{{ $t('vehicles.ownershipWizardModal.longTerm') }}</div>
                </div>
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
            <div>
              <label for="own-start-date" class="block text-xs font-semibold text-slate-300 mb-1">
                {{ isLease ? $t('vehicles.ownershipWizardModal.leaseStart') : $t('vehicles.ownershipWizardModal.purchaseDate') }} <span class="text-rose-400">*</span>
              </label>
              <AppDatePicker id="own-start-date" v-model="ownershipForm.start_date" required size="sm" />
            </div>
            <div>
              <label for="own-start-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.odometerAtTheStartKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput id="own-start-odometer" v-model="ownershipForm.start_odometer" min="0" :placeholder="$t('common.example', { value: '0' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
            </div>
          </div>
        </div>

        <!-- STEP 2: Financial Terms -->
        <div v-show="currentOwnershipStep === 2" class="space-y-4">
          <!-- Purchase terms -->
          <div v-if="isPurchase" class="space-y-3">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
              <Wallet class="w-4 h-4" />
              <span>{{ $t('vehicles.ownershipWizardModal.purchaseTerms') }}</span>
            </h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-purchase-price" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.purchasePriceInclTax', { cur: currencySymbol(currency) }) }} <span class="text-rose-400">*</span></label>
                <input id="own-purchase-price" v-model.number="ownershipForm.purchase_price" type="number" step="0.01" min="0" required :placeholder="$t('common.example', { value: '42000' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-purchase-fees" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.additionalFeesRegistrationSetUp', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-purchase-fees" v-model.number="ownershipForm.purchase_fees" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '350' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-incentives" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.bonusesAndGrants', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-incentives" v-model.number="ownershipForm.incentives" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '4000' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
          </div>

          <!-- Loan specific terms -->
          <div v-if="ownershipForm.acquisition_type === 'LOAN'" class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
              <CreditCard class="w-4 h-4" />
              <span>{{ $t('vehicles.ownershipWizardModal.loanTerms') }}</span>
            </h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-loan-amount" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.amountBorrowed', { cur: currencySymbol(currency) }) }} <span class="text-rose-400">*</span></label>
                <input id="own-loan-amount" v-model.number="ownershipForm.loan_amount" type="number" step="0.01" min="0" required :placeholder="$t('common.example', { value: '30000' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-rate" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.annualRate') }} <span class="text-rose-400">*</span></label>
                <input id="own-loan-rate" v-model.number="ownershipForm.loan_rate_pct" type="number" step="0.001" min="0" max="30" required :placeholder="$t('common.example', { value: $n(3.8) })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-duration" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.durationMonths') }} <span class="text-rose-400">*</span></label>
                <input id="own-loan-duration" v-model.number="ownershipForm.loan_duration_months" type="number" min="1" max="360" required :placeholder="$t('common.example', { value: '60' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-fees" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.arrangementFees', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-loan-fees" v-model.number="ownershipForm.loan_fees" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '200' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-loan-insurance" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.borrowerInsuranceMonth', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-loan-insurance" v-model.number="ownershipForm.loan_insurance_monthly" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '15' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>

            <!-- Loan Live Preview Card -->
            <div v-if="loanPreview" class="p-3 bg-slate-800/70 border border-indigo-500/20 rounded-xl space-y-1 text-xs">
              <div class="flex items-center justify-between text-white font-semibold">
                <span>{{ $t('vehicles.ownershipWizardModal.estimatedMonthlyPayment') }}</span>
                <span class="text-indigo-300 font-bold text-sm">{{ $t('vehicles.ownershipWizardModal.month2', { payment: formatAmount(loanPreview.payment, currency) }) }}</span>
              </div>
              <div class="flex items-center justify-between text-slate-400 text-xs">
                <span>{{ $t('vehicles.ownershipWizardModal.totalBankInterest') }}</span>
                <span>{{ formatAmount(loanPreview.totalInterest, currency) }}</span>
              </div>
              <div class="flex items-center justify-between text-slate-400 text-xs">
                <span>{{ $t('vehicles.ownershipWizardModal.totalCostOfTheLoan') }}</span>
                <span class="text-slate-200 font-medium">{{ formatAmount(loanPreview.totalCost, currency) }}</span>
              </div>
            </div>
          </div>

          <!-- Lease specific terms -->
          <div v-if="isLease" class="space-y-3">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
              <RefreshCw class="w-4 h-4" />
              <span>{{ $t('vehicles.ownershipWizardModal.leaseTerms', { acquisition_type: ownershipForm.acquisition_type }) }}</span>
            </h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-lease-down" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.downPaymentIncreasedFirstRent', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-lease-down" v-model.number="ownershipForm.lease_down_payment" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '3000' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-rent" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.monthlyRent', { cur: currencySymbol(currency) }) }} <span class="text-rose-400">*</span></label>
                <input id="own-lease-rent" v-model.number="ownershipForm.lease_monthly_rent" type="number" step="0.01" min="0" required :placeholder="$t('common.example', { value: '450' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-duration" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.durationMonths') }} <span class="text-rose-400">*</span></label>
                <input id="own-lease-duration" v-model.number="ownershipForm.lease_duration_months" type="number" min="1" max="360" required :placeholder="$t('common.example', { value: '36' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-fees" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.arrangementFees', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-lease-fees" v-model.number="ownershipForm.lease_fees" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '150' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-deposit" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.refundableSecurityDeposit', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-lease-deposit" v-model.number="ownershipForm.lease_deposit" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '500' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>

            <!-- Lease Live Preview Card -->
            <div v-if="leasePreview" class="p-3 bg-slate-800/70 border border-indigo-500/20 rounded-xl space-y-1 text-xs">
              <div class="flex items-center justify-between text-white font-semibold">
                <span>{{ $t('vehicles.ownershipWizardModal.totalRentCommitted') }}</span>
                <span class="text-indigo-300 font-bold text-sm">{{ formatAmount(leasePreview.total, currency) }}</span>
              </div>
              <div class="flex items-center justify-between text-slate-400 text-xs">
                <span>{{ $t('vehicles.ownershipWizardModal.averageSmoothedOverTheTerm') }}</span>
                <span>{{ $t('vehicles.ownershipWizardModal.month', { perMonth: formatAmount(leasePreview.perMonth, currency) }) }}</span>
              </div>
              <div v-if="leasePreview.totalKm" class="flex items-center justify-between text-slate-400 text-xs">
                <span>{{ $t('vehicles.ownershipWizardModal.totalMileageIncludedInThe') }}</span>
                <span class="text-slate-200 font-medium">{{ formatDistance(leasePreview.totalKm) }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- STEP 3: Usage, Buyout, Holding & End of Ownership -->
        <div v-show="currentOwnershipStep === 3" class="space-y-4">
          <!-- Lease Conditions & Buyout -->
          <div v-if="isLease" class="space-y-3">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">{{ $t('vehicles.ownershipWizardModal.mileageAllowanceAndInclusions') }}</h4>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <label for="own-lease-allowance" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.mileageAllowanceKmYear', { unit: distanceUnit() }) }}</label>
                <DistanceInput id="own-lease-allowance" v-model="ownershipForm.lease_km_allowance_per_year" min="0" :placeholder="$t('common.example', { value: formatDistanceValue(15000) })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-excess" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.pricePerExtraKmKm', { unit: distanceUnit(), cur: currencySymbol(currency) }) }}</label>
                <DistanceInput kind="per-distance" :digits="3" id="own-lease-excess" v-model="ownershipForm.lease_excess_km_price" step="0.001" min="0" max="5" :placeholder="$t('common.example', { value: formatPerDistanceValue(0.15, 2) })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-lease-end-fees" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.estimatedReturnFees', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-lease-end-fees" v-model.number="ownershipForm.lease_end_fees_estimate" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '400' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>

            <!-- Included Services -->
            <div class="p-3 bg-slate-800/50 rounded-xl border border-slate-800 space-y-2">
              <div class="text-xs font-semibold text-slate-300">{{ $t('vehicles.ownershipWizardModal.servicesIncludedInTheContract') }}</div>
              <div class="flex flex-wrap gap-x-6 gap-y-2">
                <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-300 hover:text-white">
                  <input id="own-incl-maintenance" v-model="ownershipForm.lease_includes_maintenance" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500 focus:ring-0" />
                  <span>{{ $t('vehicles.ownershipWizardModal.maintenanceIncluded') }}</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-300 hover:text-white">
                  <input id="own-incl-insurance" v-model="ownershipForm.lease_includes_insurance" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500 focus:ring-0" />
                  <span>{{ $t('vehicles.ownershipWizardModal.insuranceIncluded') }}</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer text-xs text-slate-300 hover:text-white">
                  <input id="own-incl-tires" v-model="ownershipForm.lease_includes_tires" type="checkbox" class="rounded border-slate-700 bg-slate-800 text-indigo-500 focus:ring-0" />
                  <span>{{ $t('vehicles.ownershipWizardModal.tiresIncluded') }}</span>
                </label>
              </div>
            </div>

            <!-- LOA Buyout Option -->
            <div v-if="ownershipForm.acquisition_type === 'LOA'" class="pt-2 border-t border-slate-800/80">
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label for="own-lease-option" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.residualPurchaseOptionInclTax', { cur: currencySymbol(currency) }) }}</label>
                  <input id="own-lease-option" v-model.number="ownershipForm.lease_purchase_option_price" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '18000' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div>
                  <label for="own-option-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.optionExercisedOnEmptyIf') }}</label>
                  <AppDatePicker id="own-option-date" v-model="ownershipForm.option_exercised_date" size="sm" :clearable="true" />
                </div>
              </div>
            </div>
          </div>

          <!-- Depreciation for owned vehicles -->
          <div v-if="isOwnedPhase" class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">{{ $t('vehicles.ownershipWizardModal.depreciationAndPlannedHolding') }}</h4>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label for="own-resale" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.estimatedPlannedResale', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-resale" v-model.number="ownershipForm.expected_resale_value" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '22000' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
              <div>
                <label for="own-holding" class="block text-xs font-semibold text-slate-300 mb-1">
                  {{ ownershipForm.acquisition_type === 'LOA' && ownershipForm.option_exercised_date ? $t('vehicles.ownershipWizardModal.holdingAfterBuyout') : $t('vehicles.ownershipWizardModal.holdingTotal') }}
                </label>
                <input id="own-holding" v-model.number="ownershipForm.expected_holding_months" type="number" min="1" max="360" :placeholder="$t('common.example', { value: '48' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
          </div>

          <!-- Clôture / End of Contract -->
          <div class="space-y-3 pt-3 border-t border-slate-800">
            <h4 class="text-xs font-bold text-indigo-400 uppercase tracking-wider">{{ $t('vehicles.ownershipWizardModal.actualClosingIfFinished') }}</h4>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label for="own-end-date" class="block text-xs font-semibold text-slate-300 mb-1">
                  {{ isLease && !ownershipForm.option_exercised_date ? $t('vehicles.ownershipWizardModal.returnedOn') : $t('vehicles.ownershipWizardModal.soldOn') }}
                </label>
                <AppDatePicker id="own-end-date" v-model="ownershipForm.end_date" size="sm" :clearable="true" />
              </div>
              <div v-if="isOwnedPhase">
                <label for="own-sale-price" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('vehicles.ownershipWizardModal.actualResalePrice', { cur: currencySymbol(currency) }) }}</label>
                <input id="own-sale-price" v-model.number="ownershipForm.sale_price" type="number" step="0.01" min="0" :placeholder="$t('common.example', { value: '21500' })" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer: Navigation Controls -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-between items-center gap-2 shrink-0 bg-slate-900/95">
        <div>
          <button
            v-if="ownership"
            type="button"
            @click="handleDeleteOwnership"
            class="px-4 py-2 bg-slate-800 hover:bg-rose-900/40 text-rose-400 text-xs font-semibold rounded-xl transition-colors"
          >
            {{ $t('vehicles.ownershipWizardModal.deleteTheContract') }}
          </button>
        </div>

        <div class="flex items-center gap-2">
          <button
            type="button"
            @click="open = false"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
          >
            {{ $t('common.cancel') }}
          </button>

          <button
            v-if="currentOwnershipStep > 1"
            type="button"
            @click="prevOwnershipStep"
            class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors flex items-center gap-1.5"
          >
            <ChevronLeft class="w-4 h-4" />
            <span>{{ $t('vehicles.ownershipWizardModal.previous') }}</span>
          </button>

          <button
            v-if="currentOwnershipStep < 3"
            type="button"
            @click="nextOwnershipStep"
            class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl transition-colors flex items-center gap-1.5"
          >
            <span>{{ $t('vehicles.ownershipWizardModal.next') }}</span>
            <ChevronRight class="w-4 h-4" />
          </button>

          <button
            v-else
            type="button"
            @click="handleSaveOwnership"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors flex items-center gap-1.5"
          >
            <Check class="w-4 h-4" />
            <span>{{ $t('vehicles.ownershipWizardModal.saveTheContract') }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
