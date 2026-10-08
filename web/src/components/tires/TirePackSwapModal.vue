<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Snowflake } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { getTireSelectLabel } from '@/utils/tires'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

const props = defineProps<{ vehicleId: string; storageTires: any[]; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const packSwapForm = ref({
  odometer: 0,
  tires: {
    FL: '',
    FR: '',
    RL: '',
    RR: '',
  },
})

// Pre-fill with the first 4 storage tires
watch(open, (isOpen) => {
  if (!isOpen) return
  packSwapForm.value.odometer = Math.round(props.currentOdometer || 0)
  const st = props.storageTires
  packSwapForm.value.tires.FL = st[0]?.tire.id || ''
  packSwapForm.value.tires.FR = st[1]?.tire.id || ''
  packSwapForm.value.tires.RL = st[2]?.tire.id || ''
  packSwapForm.value.tires.RR = st[3]?.tire.id || ''
})

const { pending: submitting, run: runOnce } = useSubmit()

async function handlePackSwapSubmitAction() {
  if (!props.vehicleId) return
  const selectedIDs = [
    packSwapForm.value.tires.FL,
    packSwapForm.value.tires.FR,
    packSwapForm.value.tires.RL,
    packSwapForm.value.tires.RR,
  ].filter(Boolean)

  if (selectedIDs.length !== 4) {
    showAlert(t('tires.tirePackSwapModal.selectFour'), t('tires.tirePackSwapModal.selectionRequired'), 'warning')
    return
  }

  try {
    await api.quickRotateTires(props.vehicleId, {
      mode: 'SWAP_PACK',
      odometer: packSwapForm.value.odometer,
      swap_with_pack_tire_ids: selectedIDs,
    })
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handlePackSwapSubmit = () => runOnce(handlePackSwapSubmitAction)
</script>

<template>
  <ModalShell
    v-model:open="open"
    :title="$t('tires.tirePackSwapModal.seasonalSwapFullSetChange')"
    :icon="Snowflake"
    icon-class="text-info-400"
    body-class="space-y-4 text-xs"
    footer-class="items-center justify-end gap-2"
  >
    <div>
      <label for="tire-pack-swap-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tirePackSwapModal.odometerAtTheSwapKm', { unit: distanceUnit() }) }}</label>
      <DistanceInput id="tire-pack-swap-odometer"
        v-model="packSwapForm.odometer"
        class="field"
      />
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
      <div>
        <label for="tire-pack-swap-tires-fl" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tirePackSwapModal.frontLeftFl') }}</label>
        <select id="tire-pack-swap-tires-fl"
          v-model="packSwapForm.tires.FL"
          class="field"
        >
          <option value="">{{ $t('tires.tirePackSwapModal.chooseATire') }}</option>
          <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
            {{ getTireSelectLabel(t) }}
          </option>
        </select>
      </div>

      <div>
        <label for="tire-pack-swap-tires-fr" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tirePackSwapModal.frontRightFr') }}</label>
        <select id="tire-pack-swap-tires-fr"
          v-model="packSwapForm.tires.FR"
          class="field"
        >
          <option value="">{{ $t('tires.tirePackSwapModal.chooseATire') }}</option>
          <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
            {{ getTireSelectLabel(t) }}
          </option>
        </select>
      </div>

      <div>
        <label for="tire-pack-swap-tires-rl" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tirePackSwapModal.rearLeftRl') }}</label>
        <select id="tire-pack-swap-tires-rl"
          v-model="packSwapForm.tires.RL"
          class="field"
        >
          <option value="">{{ $t('tires.tirePackSwapModal.chooseATire') }}</option>
          <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
            {{ getTireSelectLabel(t) }}
          </option>
        </select>
      </div>

      <div>
        <label for="tire-pack-swap-tires-rr" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tirePackSwapModal.rearRightRr') }}</label>
        <select id="tire-pack-swap-tires-rr"
          v-model="packSwapForm.tires.RR"
          class="field"
        >
          <option value="">{{ $t('tires.tirePackSwapModal.chooseATire') }}</option>
          <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
            {{ getTireSelectLabel(t) }}
          </option>
        </select>
      </div>
    </div>
    <template #footer>
      <button
        type="button"
        @click="open = false"
        class="btn btn-lg btn-secondary"
      >
        {{ $t('common.cancel') }}
      </button>
      <button :disabled="submitting"
        type="button"
        @click="handlePackSwapSubmit"
        class="btn btn-lg btn-primary"
      >
        {{ $t('tires.tirePackSwapModal.confirmTheRotation') }}
      </button>
    </template>
  </ModalShell>
</template>