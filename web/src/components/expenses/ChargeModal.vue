<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useDocumentAttach } from '@/composables/useDocumentAttach'
import { useVehicleStore } from '@/stores/vehicle'
import { Zap, X, Paperclip, FileText, Eye, UploadCloud, Calculator } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { currencyPayload, formatDate, toLocalDateTimeInput } from '@/utils/expenses'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import ChargeAmountFields from '@/components/charges/ChargeAmountFields.vue'
import { loadMemory, rememberCharge } from '@/utils/quickAdd'
import PublicChargeCalculatorModal from '@/components/expenses/PublicChargeCalculatorModal.vue'
import { formatNumber } from '@/utils/numbers'
import { useSubmit } from '@/composables/useSubmit'
import { useSessionTariffCost } from '@/composables/useSessionTariffCost'
import { sessionPriceRequest } from '@/utils/tariffSession'

// Records a charge made outside TeslaMate, or completes / corrects the cost of `editing`.
const props = defineProps<{ vehicleId: string; editing: any | null; documents: ExpenseDocumentHeader[]; currentOdometer: number }>()
const emit = defineEmits<{
  saved: []
  'document-added': [doc: ExpenseDocumentHeader]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const vehicleStore = useVehicleStore()
const { isUploadingDocument, onSelectExistingDoc, onFileInputChange } = useDocumentAttach(
  () => props.vehicleId,
  () => props.documents,
  (doc) => emit('document-added', doc)
)

const editingCharge = computed(() => props.editing)
const baseCurrency = computed(() => vehicleStore.currency)

const chargeForm = ref({
  date: toLocalDateTimeInput(new Date()),
  end_date: '',
  kwh_added: '',
  cost: '',
  currency: baseCurrency.value,
  fx_rate: '',
  address: '',
  odometer: '',
  notes: '',
  document_id: null as string | null,
  document_filename: null as string | null,
})

// The vehicle's tariff prices a session that has no cost yet from its hours, whether it was typed here or synchronised
const pricesFromTariff = computed(() => open.value && !(editingCharge.value && editingCharge.value.cost !== null && editingCharge.value.cost !== undefined))
const tariffCost = useSessionTariffCost(
  () => sessionPriceRequest(props.vehicleId, chargeForm.value.date, chargeForm.value.end_date, chargeForm.value.kwh_added),
  pricesFromTariff,
)

const showPublicCalc = ref(false)
const amounts = ref<InstanceType<typeof ChargeAmountFields> | null>(null)
const suggestedPrice = computed(() => (editingCharge.value ? undefined : loadMemory(props.vehicleId).pricePerKwh))

function handleApplyPublicCalc(cost: number, calculatedKwh?: number, summaryNote?: string) {
  amounts.value?.setCost(cost)
  if (calculatedKwh && (!chargeForm.value.kwh_added || Number(chargeForm.value.kwh_added) <= 0)) {
    chargeForm.value.kwh_added = calculatedKwh.toFixed(3)
  }
  if (summaryNote) {
    if (chargeForm.value.notes) {
      chargeForm.value.notes += ' | ' + summaryNote
    } else {
      chargeForm.value.notes = summaryNote
    }
  }
}

watch(open, (isOpen) => {
  if (!isOpen) return
  const c = props.editing
  if (!c) {
    chargeForm.value = {
      date: toLocalDateTimeInput(new Date()),
      end_date: '',
      kwh_added: '',
      cost: '',
      currency: baseCurrency.value,
      fx_rate: '',
      address: '',
      odometer: props.currentOdometer ? String(Math.round(props.currentOdometer)) : '',
      notes: '',
      document_id: null,
      document_filename: null,
    }
  } else {
    chargeForm.value = {
      date: toLocalDateTimeInput(new Date(c.date)),
      end_date: c.end_date ? toLocalDateTimeInput(new Date(c.end_date)) : '',
      kwh_added: String(c.kwh_added),
      cost: c.cost !== null && c.cost !== undefined ? String(c.cost) : '',
      currency: c.currency || baseCurrency.value,
      fx_rate: c.fx_rate ? String(c.fx_rate) : '',
      address: c.address || '',
      odometer: c.odometer ? String(Math.round(c.odometer)) : '',
      notes: c.notes || '',
      document_id: c.document_id || null,
      document_filename: c.document_filename || null,
    }
  }
})

const { pending: submitting, run: runOnce } = useSubmit()

async function handleSaveChargeAction() {
  if (!props.vehicleId) return
  const payload = {
    date: new Date(chargeForm.value.date).toISOString(),
    end_date: chargeForm.value.end_date ? new Date(chargeForm.value.end_date).toISOString() : null,
    kwh_added: Number(chargeForm.value.kwh_added),
    cost: Number(chargeForm.value.cost),
    ...currencyPayload(chargeForm.value, baseCurrency.value),
    address: chargeForm.value.address || null,
    odometer: chargeForm.value.odometer ? Number(chargeForm.value.odometer) : null,
    notes: chargeForm.value.notes || null,
    document_id: chargeForm.value.document_id || null,
  }
  try {
    if (editingCharge.value) {
      await api.updateCharge(props.vehicleId, editingCharge.value.id, payload)
    } else {
      await api.createCharge(props.vehicleId, payload)
      rememberCharge(props.vehicleId, { kwh: payload.kwh_added, cost: payload.cost, address: payload.address })
    }
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleSaveCharge = () => runOnce(handleSaveChargeAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-modal bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="min-w-0 pr-2">
          <h3 class="text-base font-bold text-white flex items-center gap-2 truncate">
            <Zap class="w-5 h-5 text-info-400 shrink-0" />
            {{ !editingCharge ? $t('expenses.chargeModal.newCharge') : editingCharge.is_manual ? $t('expenses.chargeModal.editCharge') : $t('expenses.chargeModal.chargeCost') }}
          </h3>
          <p v-if="editingCharge && !editingCharge.is_manual" class="text-xs text-slate-400 mt-1">
            {{ $t('expenses.chargeModal.teslamateChargeOfKwhThe', { date: formatDate(editingCharge.date), kwh_added: formatNumber(editingCharge.kwh_added, 2) }) }}
          </p>
        </div>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors shrink-0" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="charge-modal-form" @submit.prevent="handleSaveCharge" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <template v-if="!editingCharge || editingCharge.is_manual">
          <div>
            <label for="charge-form-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargeModal.start') }}</label>
            <AppDatePicker id="charge-form-date" v-model="chargeForm.date" enable-time-picker required size="xs" />
          </div>
          <div>
            <label for="charge-form-end" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargeModal.endOptional') }}</label>
            <AppDatePicker id="charge-form-end" v-model="chargeForm.end_date" enable-time-picker clearable size="xs" />
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="charge-form-address" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargeModal.placeOptional') }}</label>
              <input id="charge-form-address" v-model="chargeForm.address" :placeholder="$t('expenses.chargeModal.chargerHome')" class="field" />
            </div>
            <div>
              <label for="charge-form-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargeModal.odometerOptional') }}</label>
              <DistanceInput text id="charge-form-odometer" v-model="chargeForm.odometer" inputmode="numeric" min="0" class="field" />
            </div>
          </div>
        </template>

        <ChargeAmountFields
          ref="amounts"
          v-model:kwh="chargeForm.kwh_added"
          v-model:cost="chargeForm.cost"
          id-prefix="charge-form"
          required
          :hide-energy="!!editingCharge && !editingCharge.is_manual"
          :currency="chargeForm.currency"
          :suggested-price="suggestedPrice"
          :tariff="tariffCost"
        >
          <template #cost-action>
            <button
              type="button"
              @click="showPublicCalc = true"
              class="inline-flex items-center gap-1 text-xs text-blue-400 hover:text-blue-300 font-medium"
            >
              <Calculator class="w-3.5 h-3.5" />
              {{ $t('tariffs.publicModal.openCalculator') }}
            </button>
          </template>
        </ChargeAmountFields>
        <div v-if="chargeForm.currency !== baseCurrency">
          <label for="charge-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargeModal.conversionRate1', { currency: chargeForm.currency, base: baseCurrency }) }}</label>
          <NumberInput text id="charge-form-fx-rate" v-model="chargeForm.fx_rate" min="0.000001" required class="field" />
        </div>
        <div>
          <label for="charge-form-notes" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargeModal.notesOptional') }}</label>
          <input id="charge-form-notes" v-model="chargeForm.notes" class="field" />
        </div>

        <!-- Justificatif / Facture -->
        <div class="space-y-2 bg-slate-800/40 p-3 rounded-xl border border-slate-700/60">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <Paperclip class="w-3.5 h-3.5 text-indigo-400" />
              {{ $t('expenses.chargeModal.receiptInvoice') }}
            </span>
            <span v-if="chargeForm.document_id" class="text-xs text-success-400 font-medium">{{ $t('expenses.chargeModal.linked') }}</span>
          </div>

          <div v-if="chargeForm.document_id" class="flex items-center justify-between p-2.5 bg-slate-900 border border-indigo-500/30 rounded-xl">
            <div class="flex items-center gap-2 min-w-0">
              <FileText class="w-4 h-4 text-indigo-400 shrink-0" />
              <span class="text-xs text-white truncate font-medium">{{ chargeForm.document_filename || $t('expenses.linkedInvoice') }}</span>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              <button
                type="button"
                @click="emit('view-document', chargeForm.document_id, chargeForm.document_filename, false)"
                class="tap p-1 text-slate-400 hover:text-indigo-400 rounded-lg hover:bg-slate-800"
                :title="$t('expenses.chargeModal.viewTheDocument')" :aria-label="$t('expenses.chargeModal.viewTheDocument')"
              >
                <Eye class="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                @click="chargeForm.document_id = null; chargeForm.document_filename = null"
                class="tap p-1 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800"
                :title="$t('expenses.chargeModal.detachTheReceipt')" :aria-label="$t('expenses.chargeModal.detachTheReceipt')"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div v-else class="space-y-2">
            <div class="flex flex-col sm:flex-row gap-2">
              <div class="flex-1" v-if="documents.length > 0">
                <label for="charge-existing-doc" class="sr-only">{{ $t('expenses.chargeModal.pickAnExistingInvoice') }}</label>
                <select
                  id="charge-existing-doc"
                  class="field text-slate-300"
                  @change="(e: any) => onSelectExistingDoc(e.target.value, chargeForm)"
                >
                  <option value="">{{ $t('expenses.chargeModal.attachAnExistingInvoice') }}</option>
                  <option v-for="d in documents" :key="d.id" :value="d.id">
                    {{ d.filename }} ({{ formatDate(d.created_at) }})
                  </option>
                </select>
              </div>
              <label class="cursor-pointer px-3 py-1.5 bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 rounded-xl text-xs font-medium flex items-center justify-center gap-1.5 transition-colors">
                <UploadCloud class="w-3.5 h-3.5" />
                <span>{{ isUploadingDocument ? $t('expenses.uploading') : $t('expenses.newFile') }}</span>
                <input
                  type="file"
                  accept=".pdf,image/png,image/jpeg,image/webp"
                  class="hidden"
                  :disabled="isUploadingDocument"
                  @change="(e: any) => onFileInputChange(e, chargeForm)"
                />
              </label>
            </div>
            <p class="text-xs text-slate-400">
              {{ $t('expenses.chargeModal.pdfOrImageSuperchargerReceipt') }}
            </p>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting" type="submit" form="charge-modal-form" class="btn btn-lg btn-primary">
          {{ $t('common.save') }}
        </button>
      </div>
    </div>
  </div>

  <PublicChargeCalculatorModal
    v-model:open="showPublicCalc"
    :currency="chargeForm.currency"
    :initial-kwh="chargeForm.kwh_added ? Number(chargeForm.kwh_added) : null"
    @apply="handleApplyPublicCalc"
  />
</template>
