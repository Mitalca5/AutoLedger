<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Plus, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { currencySymbol } from '@/currency'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

const props = defineProps<{ vehicleId: string; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const vehicleStore = useVehicleStore()

const addType = ref<'SET_4' | 'SET_4_STORAGE' | 'SET_2_FRONT' | 'SET_2_REAR' | 'SET_2_STORAGE' | 'SINGLE'>('SET_4')
const isTotalPrice = ref(true)

const addTireForm = ref({
  brand: '',
  model: '',
  dimension: '',
  season: 'SUMMER',
  purchase_date: todayIso(),
  total_price: 880,
  unit_price: 220,
  initial_depth_mm: 8.0,
  min_legal_depth_mm: 1.6,
  dot_code: '',
  mounted_odometer: 0,
  accumulated_distance_km: 0,
  estimated_lifespan_km: 45000,
  current_position: 'FL',
})

// Mounted tires start at the vehicle's current odometer
watch(open, (isOpen) => {
  if (!isOpen) return
  if (props.currentOdometer) addTireForm.value.mounted_odometer = Math.round(props.currentOdometer)
})

const { pending: submitting, run: runOnce } = useSubmit()

async function handleCreateTiresAction() {
  if (!props.vehicleId) return
  if (!addTireForm.value.brand || !addTireForm.value.model || !addTireForm.value.dimension) {
    showAlert(t('tires.tireAddModal.requiredFields'), t('tires.tireAddModal.requiredFieldsTitle'), 'warning')
    return
  }

  try {
    if (addType.value === 'SINGLE') {
      await api.createTire(props.vehicleId, {
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        purchase_price: addTireForm.value.unit_price,
        current_position: addTireForm.value.current_position,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addTireForm.value.current_position !== 'STORAGE' ? addTireForm.value.mounted_odometer : null,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    } else {
      await api.batchCreateTires(props.vehicleId, {
        type: addType.value,
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        total_price: isTotalPrice.value ? addTireForm.value.total_price : 0,
        unit_price: !isTotalPrice.value ? addTireForm.value.unit_price : 0,
        initial_depth_mm: addTireForm.value.initial_depth_mm,
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addType.value.includes('STORAGE') ? null : addTireForm.value.mounted_odometer,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: addTireForm.value.estimated_lifespan_km,
      })
    }

    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleCreateTires = () => runOnce(handleCreateTiresAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Plus class="w-5 h-5 text-rose-500" />
          {{ $t('tires.tireAddModal.addTires') }}
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

      <!-- Add Type Selection -->
      <div class="space-y-1.5">
        <span class="block text-xs font-semibold text-slate-300">{{ $t('tires.tireAddModal.recordingFormat') }}</span>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
          <button
            type="button"
            @click="addType = 'SET_4'"
            :aria-pressed="addType === 'SET_4'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_4' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            {{ $t('tires.tireAddModal.fullSetFitted4Tires') }}
          </button>
          <button
            type="button"
            @click="addType = 'SET_4_STORAGE'"
            :aria-pressed="addType === 'SET_4_STORAGE'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_4_STORAGE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            {{ $t('tires.tireAddModal.setInStorage4Winter') }}
          </button>
          <button
            type="button"
            @click="addType = 'SET_2_FRONT'"
            :aria-pressed="addType === 'SET_2_FRONT'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_2_FRONT' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            {{ $t('tires.tireAddModal.frontAxleFitted2Tires') }}
          </button>
          <button
            type="button"
            @click="addType = 'SET_2_REAR'"
            :aria-pressed="addType === 'SET_2_REAR'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_2_REAR' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            {{ $t('tires.tireAddModal.rearAxleFitted2Tires') }}
          </button>
          <button
            type="button"
            @click="addType = 'SET_2_STORAGE'"
            :aria-pressed="addType === 'SET_2_STORAGE'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SET_2_STORAGE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            {{ $t('tires.tireAddModal.pairInStorage2Tires') }}
          </button>
          <button
            type="button"
            @click="addType = 'SINGLE'"
            :aria-pressed="addType === 'SINGLE'"
            class="p-2.5 rounded-xl border text-left transition-all"
            :class="addType === 'SINGLE' ? 'bg-rose-500/15 text-rose-300 border-rose-500/40 font-bold' : 'bg-slate-800/60 text-slate-400 border-slate-700'"
          >
            {{ $t('tires.tireAddModal.singleTireOnePurchase') }}
          </button>
        </div>
      </div>

      <!-- Position (single tire only) -->
      <div v-if="addType === 'SINGLE'" class="space-y-1">
        <label for="tire-add-tire-position" class="block text-xs font-semibold text-slate-400">{{ $t('tires.tireAddModal.position') }}</label>
        <select id="tire-add-tire-position"
          v-model="addTireForm.current_position"
          class="field"
        >
          <option value="FL">{{ $t('tires.tireAddModal.frontLeftFl') }}</option>
          <option value="FR">{{ $t('tires.tireAddModal.frontRightFr') }}</option>
          <option value="RL">{{ $t('tires.tireAddModal.rearLeftRl') }}</option>
          <option value="RR">{{ $t('tires.tireAddModal.rearRightRr') }}</option>
          <option value="STORAGE">{{ $t('tires.tireAddModal.garageStockNotFitted') }}</option>
        </select>
      </div>

      <!-- Brand, Model, Dimension, Season -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label for="tire-add-tire-brand" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.brand') }}</label>
          <input id="tire-add-tire-brand"
            v-model="addTireForm.brand"
            type="text"
            :placeholder="$t('tires.tireAddModal.brand')"
            class="field"
          />
        </div>
        <div>
          <label for="tire-add-tire-model" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.model') }}</label>
          <input id="tire-add-tire-model"
            v-model="addTireForm.model"
            type="text"
            :placeholder="$t('tires.tireAddModal.model')"
            class="field"
          />
        </div>
        <div>
          <label for="tire-add-tire-dimension" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.dimension') }}</label>
          <input id="tire-add-tire-dimension"
            v-model="addTireForm.dimension"
            type="text"
            placeholder="235/45 R18 98Y"
            class="field font-mono"
          />
        </div>
        <div>
          <label for="tire-add-tire-season" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.season') }}</label>
          <select id="tire-add-tire-season"
            v-model="addTireForm.season"
            class="field"
          >
            <option value="SUMMER">{{ $t('tires.tireAddModal.summer') }}</option>
            <option value="WINTER">{{ $t('tires.tireAddModal.winter') }}</option>
            <option value="ALL_SEASON">{{ $t('tires.tireAddModal.allSeason') }}</option>
          </select>
        </div>
      </div>

      <!-- Pricing & Lifespan -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 bg-slate-950/60 p-3.5 rounded-2xl border border-slate-800">
        <div>
          <div class="flex items-center justify-between mb-1">
            <label for="tire-add-tire-total-price" class="text-xs font-semibold text-slate-400">
              {{ isTotalPrice ? $t('tires.tireAddModal.totalPrice', { cur: currencySymbol(vehicleStore.currency) }) : $t('tires.tireAddModal.pricePerTire', { cur: currencySymbol(vehicleStore.currency) }) }}
            </label>
            <button
              type="button"
              @click="isTotalPrice = !isTotalPrice"
              class="text-xs text-rose-400 hover:text-rose-300 underline"
            >
              {{ $t('tires.tireAddModal.switchTo', { mode: isTotalPrice ? $t('tires.tireAddModal.unitPrice') : $t('tires.tireAddModal.totalPriceMode') }) }}
            </button>
          </div>
          <input id="tire-add-tire-total-price"
            v-if="isTotalPrice"
            v-model.number="addTireForm.total_price"
            type="number"
            step="10"
            class="field text-success-400 font-bold"
          />
          <input id="tire-add-tire-total-price"
            v-else
            v-model.number="addTireForm.unit_price"
            type="number"
            step="5"
            class="field text-success-400 font-bold"
          />
        </div>

        <div>
          <label for="tire-add-tire-estimated-lifespan-km" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.estimatedLifespanKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput whole id="tire-add-tire-estimated-lifespan-km"
            v-model="addTireForm.estimated_lifespan_km"
            step="5000"
            class="field"
          />
        </div>
      </div>

      <!-- Km already driven (second-hand) -->
      <div>
        <label for="tire-add-tire-accumulated-distance-km" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.kmAlreadyDrivenIfUsed', { unit: distanceUnit() }) }}</label>
        <DistanceInput id="tire-add-tire-accumulated-distance-km"
          v-model="addTireForm.accumulated_distance_km"
          class="field"
        />
      </div>

      <!-- Date & Sculptures -->
      <div class="grid grid-cols-3 gap-3">
        <div>
          <label for="tire-add-tire-purchase-date" class="block text-xs text-slate-400 mb-1">{{ $t('tires.tireAddModal.purchaseDate') }}</label>
          <AppDatePicker
            id="tire-add-tire-purchase-date"
            v-model="addTireForm.purchase_date"
            size="xs"
            :clearable="true"
          />
        </div>
        <div>
          <label for="tire-add-tire-initial-depth-mm" class="block text-xs text-slate-400 mb-1">{{ $t('tires.tireAddModal.newTreadMm') }}</label>
          <input id="tire-add-tire-initial-depth-mm"
            v-model.number="addTireForm.initial_depth_mm"
            type="number"
            step="0.1"
            class="field"
          />
        </div>
        <div>
          <label for="tire-add-tire-min-legal-depth-mm" class="block text-xs text-slate-400 mb-1">{{ $t('tires.tireAddModal.legalWearIndicatorMm') }}</label>
          <input id="tire-add-tire-min-legal-depth-mm"
            v-model.number="addTireForm.min_legal_depth_mm"
            type="number"
            step="0.1"
            class="field"
          />
        </div>
      </div>

      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-3 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 rounded-xl text-xs font-semibold text-slate-300 transition-colors"
        >
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting"
          type="button"
          @click="handleCreateTires"
          class="bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold px-5 py-2 rounded-xl shadow-lg shadow-rose-600/20 transition-colors"
        >
          {{ $t('tires.tireAddModal.createTheTires') }}
        </button>
      </div>
    </div>
  </div>
</template>
