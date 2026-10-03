<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Snowflake, X } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { getTireSelectLabel } from '@/utils/tires'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'

const props = defineProps<{ vehicleId: string; storageTires: any[]; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
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

async function handlePackSwapSubmit() {
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
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Snowflake class="w-4 h-4 text-sky-400" />
          {{ $t('tires.tirePackSwapModal.seasonalSwapFullSetChange') }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
        <div>
          <label for="tire-pack-swap-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tirePackSwapModal.odometerAtTheSwapKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput id="tire-pack-swap-odometer"
            v-model="packSwapForm.odometer"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
          />
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
          <div>
            <label for="tire-pack-swap-tires-fl" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tirePackSwapModal.frontLeftFl') }}</label>
            <select id="tire-pack-swap-tires-fl"
              v-model="packSwapForm.tires.FL"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
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
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
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
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
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
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-1.5 border border-slate-700"
            >
              <option value="">{{ $t('tires.tirePackSwapModal.chooseATire') }}</option>
              <option v-for="t in storageTires" :key="t.tire.id" :value="t.tire.id">
                {{ getTireSelectLabel(t) }}
              </option>
            </select>
          </div>
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
        >
          {{ $t('common.cancel') }}
        </button>
        <button
          type="button"
          @click="handlePackSwapSubmit"
          class="bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors"
        >
          {{ $t('tires.tirePackSwapModal.confirmTheRotation') }}
        </button>
      </div>
    </div>
  </div>
</template>
