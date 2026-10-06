<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { Layers, X } from 'lucide-vue-next'
import { currencySymbol } from '@/currency'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { useSubmit } from '@/composables/useSubmit'

// Merges the selected drives into a trip group, optionally with one expense (toll, parking, ferry) for the whole trip.
const props = defineProps<{ vehicleId: string; selectedDriveIds: string[]; selectedList: any[] }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const vehicleStore = useVehicleStore()

const groupName = ref('')
const tollAmount = ref<number | ''>('')
const expenseType = ref('TOLL')

const submitted = ref(false)
const nameError = computed(() => (!groupName.value.trim() ? t('drives.driveGroupModal.nameRequired') : ''))
watch(open, (isOpen) => {
  if (isOpen) submitted.value = false
})

const { pending: submitting, run: runOnce } = useSubmit()

async function handleCreateGroupAndExpenseAction() {
  if (!props.vehicleId || !props.selectedDriveIds.length) return
  submitted.value = true
  if (nameError.value) {
    document.getElementById('drive-group-name')?.focus()
    return
  }

  try {
    if (tollAmount.value && Number(tollAmount.value) > 0) {
      // Group and expense are created atomically by the backend; the expense is dated at the trip start
      const firstStart = props.selectedList.length ? new Date(props.selectedList[0].start_time).getTime() : undefined
      await api.createDriveExpense(props.vehicleId, {
        drive_ids: props.selectedDriveIds,
        type: expenseType.value,
        amount: Number(tollAmount.value),
        currency: vehicleStore.currency,
        date: new Date(firstStart || Date.now()).toISOString(),
        notes: groupName.value,
      })
    } else {
      await api.createTripGroup(props.vehicleId, {
        name: groupName.value,
        drive_ids: props.selectedDriveIds,
      })
    }

    showAlert(t('drives.driveGroupModal.created'), t('common.success'), 'success')
    open.value = false
    groupName.value = ''
    tollAmount.value = ''
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleCreateGroupAndExpense = () => runOnce(handleCreateGroupAndExpenseAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2">
          <Layers class="w-5 h-5 text-rose-400" />
          <h3 class="text-base font-bold text-white">{{ $t('drives.driveGroupModal.createATripMerge') }}</h3>
          <span class="px-2 py-0.5 bg-rose-500/10 text-rose-300 text-xs font-semibold rounded-lg border border-rose-500/20">
            {{ $t('drives.driveGroupModal.drives', { length: selectedDriveIds.length }) }}
          </span>
        </div>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">

        <div>
          <label for="drive-group-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('drives.driveGroupModal.tripGroupName') }}</label>
          <input id="drive-group-name"
            v-model="groupName"
            type="text"
            :placeholder="$t('drives.driveGroupModal.eGBrittanyHolidayOutbound')"
            :aria-invalid="submitted && !!nameError"
            :aria-describedby="submitted && nameError ? 'drive-group-name-error' : undefined"
            class="field"
          />
          <p v-if="submitted && nameError" id="drive-group-name-error" class="text-xs text-danger-400 mt-1">{{ nameError }}</p>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="drive-expense-type" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('drives.driveGroupModal.costType') }}</label>
            <select id="drive-expense-type"
              v-model="expenseType"
              class="field"
            >
              <option value="TOLL">{{ $t('drives.driveGroupModal.toll') }}</option>
              <option value="PARKING">{{ $t('drives.driveGroupModal.parking') }}</option>
              <option value="FERRY">{{ $t('drives.driveGroupModal.ferry') }}</option>
            </select>
          </div>
          <div>
            <label for="drive-toll-amount" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('drives.driveGroupModal.amount', { cur: currencySymbol(vehicleStore.currency) }) }}</label>
            <NumberInput text id="drive-toll-amount"
              v-model="tollAmount"
              placeholder="0.00"
              class="field"
            />
          </div>
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="btn btn-lg btn-secondary"
        >
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting"
          type="button"
          @click="handleCreateGroupAndExpense"
          class="btn btn-lg btn-primary"
        >
          {{ $t('drives.driveGroupModal.saveTheGroup') }}
        </button>
      </div>
    </div>
  </div>
</template>
