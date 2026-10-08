<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, nextTick, ref, watch } from 'vue'
import { Plus } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { currencySymbol } from '@/currency'
import { todayIso } from '@/utils/dates'
import { distanceUnit, formatDistanceValue } from '@/units'
import { useSubmit } from '@/composables/useSubmit'
import { validateTireForm } from '@/utils/tires'

const props = defineProps<{ vehicleId: string; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
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
  total_price: '' as number | '',
  unit_price: '' as number | '',
  initial_depth_mm: '' as number | '',
  min_legal_depth_mm: 1.6,
  dot_code: '',
  mounted_odometer: 0,
  accumulated_distance_km: 0,
  estimated_lifespan_km: '' as number | string,
  current_position: 'FL',
})

// Mounted tires start at the vehicle's current odometer
watch(open, (isOpen) => {
  if (!isOpen) return
  submitted.value = false
  if (props.currentOdometer) addTireForm.value.mounted_odometer = Math.round(props.currentOdometer)
})

const { pending: submitting, run: runOnce } = useSubmit()

const submitted = ref(false)
const priceModel = computed<number | ''>({
  get: () => (isTotalPrice.value ? addTireForm.value.total_price : addTireForm.value.unit_price),
  set: (v) => {
    if (isTotalPrice.value) addTireForm.value.total_price = v
    else addTireForm.value.unit_price = v
  },
})
const errors = computed(() => (submitted.value ? validateTireForm({ ...addTireForm.value, price: priceModel.value }) : {}))


async function handleCreateTiresAction() {
  if (!props.vehicleId) return
  submitted.value = true
  const found = validateTireForm({ ...addTireForm.value, price: priceModel.value })
  if (Object.keys(found).length > 0) {
    await nextTick()
    document.querySelector<HTMLElement>('[data-tire-add] [aria-invalid="true"]')?.focus()
    return
  }
  const totalPrice = Number(addTireForm.value.total_price) || 0
  const unitPrice = Number(addTireForm.value.unit_price) || 0

  try {
    if (addType.value === 'SINGLE') {
      await api.createTire(props.vehicleId, {
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        purchase_price: unitPrice,
        current_position: addTireForm.value.current_position,
        initial_depth_mm: Number(addTireForm.value.initial_depth_mm),
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addTireForm.value.current_position !== 'STORAGE' ? addTireForm.value.mounted_odometer : null,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: Number(addTireForm.value.estimated_lifespan_km),
      })
    } else {
      await api.batchCreateTires(props.vehicleId, {
        type: addType.value,
        brand: addTireForm.value.brand,
        model: addTireForm.value.model,
        dimension: addTireForm.value.dimension,
        season: addTireForm.value.season,
        purchase_date: addTireForm.value.purchase_date,
        total_price: isTotalPrice.value ? totalPrice : 0,
        unit_price: !isTotalPrice.value ? unitPrice : 0,
        initial_depth_mm: Number(addTireForm.value.initial_depth_mm),
        min_legal_depth_mm: addTireForm.value.min_legal_depth_mm,
        dot_code: addTireForm.value.dot_code || null,
        mounted_odometer: addType.value.includes('STORAGE') ? null : addTireForm.value.mounted_odometer,
        accumulated_distance_km: addTireForm.value.accumulated_distance_km,
        estimated_lifespan_km: Number(addTireForm.value.estimated_lifespan_km),
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
  <ModalShell
    v-model:open="open"
    :title="$t('tires.tireAddModal.addTires')"
    :icon="Plus"
    icon-class="text-rose-500"
    body-class="space-y-5"
    footer-class="items-center justify-end gap-3"
    data-tire-add
  >

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
          :aria-invalid="errors.brand ? 'true' : undefined"
          aria-describedby="tire-add-error-brand"
          type="text"
          :placeholder="$t('tires.tireAddModal.brand')"
          class="field"
        />
        <p v-if="errors.brand" id="tire-add-error-brand" class="mt-1 text-xs text-danger-400">{{ $t(errors.brand) }}</p>
      </div>
      <div>
        <label for="tire-add-tire-model" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.model') }}</label>
        <input id="tire-add-tire-model"
          v-model="addTireForm.model"
          :aria-invalid="errors.model ? 'true' : undefined"
          aria-describedby="tire-add-error-model"
          type="text"
          :placeholder="$t('tires.tireAddModal.model')"
          class="field"
        />
        <p v-if="errors.model" id="tire-add-error-model" class="mt-1 text-xs text-danger-400">{{ $t(errors.model) }}</p>
      </div>
      <div>
        <label for="tire-add-tire-dimension" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.dimension') }}</label>
        <input id="tire-add-tire-dimension"
          v-model="addTireForm.dimension"
          :aria-invalid="errors.dimension ? 'true' : undefined"
          aria-describedby="tire-add-error-dimension"
          type="text"
          placeholder="235/45 R18 98Y"
          class="field font-mono"
        />
        <p v-if="errors.dimension" id="tire-add-error-dimension" class="mt-1 text-xs text-danger-400">{{ $t(errors.dimension) }}</p>
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
        <label for="tire-add-tire-price" class="block text-xs font-semibold text-slate-400 mb-1">
          {{ isTotalPrice ? $t('tires.tireAddModal.totalPrice', { cur: currencySymbol(vehicleStore.currency) }) : $t('tires.tireAddModal.pricePerTire', { cur: currencySymbol(vehicleStore.currency) }) }}
        </label>
        <input id="tire-add-tire-price"
          v-model.number="priceModel"
          type="number"
          min="0"
          :step="isTotalPrice ? 10 : 5"
          placeholder="0"
          :aria-invalid="errors.price ? 'true' : undefined"
          aria-describedby="tire-add-error-price"
          class="field text-success-400 font-bold"
        />
        <p v-if="errors.price" id="tire-add-error-price" class="mt-1 text-xs text-danger-400">{{ $t(errors.price) }}</p>
        <button
          type="button"
          @click="isTotalPrice = !isTotalPrice"
          class="mt-1 text-xs text-rose-400 hover:text-rose-300 underline"
        >
          {{ $t('tires.tireAddModal.switchTo', { mode: isTotalPrice ? $t('tires.tireAddModal.unitPrice') : $t('tires.tireAddModal.totalPriceMode') }) }}
        </button>
      </div>

      <div>
        <label for="tire-add-tire-estimated-lifespan-km" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.estimatedLifespanKm', { unit: distanceUnit() }) }}</label>
        <DistanceInput whole id="tire-add-tire-estimated-lifespan-km"
          v-model="addTireForm.estimated_lifespan_km"
          step="5000"
          :placeholder="$t('common.example', { value: formatDistanceValue(45000) })"
          :aria-invalid="errors.estimated_lifespan_km ? 'true' : undefined"
          aria-describedby="tire-add-error-lifespan"
          class="field"
        />
        <p v-if="errors.estimated_lifespan_km" id="tire-add-error-lifespan" class="mt-1 text-xs text-danger-400">{{ $t(errors.estimated_lifespan_km) }}</p>
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
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
      <div>
        <label for="tire-add-tire-purchase-date" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.purchaseDate') }}</label>
        <AppDatePicker
          id="tire-add-tire-purchase-date"
          v-model="addTireForm.purchase_date"
          :clearable="true"
        />
      </div>
      <div>
        <label for="tire-add-tire-initial-depth-mm" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.newTreadMm') }}</label>
        <NumberInput id="tire-add-tire-initial-depth-mm"
          v-model="addTireForm.initial_depth_mm"
          :placeholder="$t('common.example', { value: $n(8) })"
          :aria-invalid="errors.initial_depth_mm ? 'true' : undefined"
          aria-describedby="tire-add-error-depth"
          class="field"
        />
        <p v-if="errors.initial_depth_mm" id="tire-add-error-depth" class="mt-1 text-xs text-danger-400">{{ $t(errors.initial_depth_mm) }}</p>
      </div>
      <div>
        <label for="tire-add-tire-min-legal-depth-mm" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('tires.tireAddModal.legalWearIndicatorMm') }}</label>
        <NumberInput id="tire-add-tire-min-legal-depth-mm"
          v-model="addTireForm.min_legal_depth_mm"
          class="field"
        />
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
        @click="handleCreateTires"
        class="btn btn-lg btn-primary"
      >
        {{ $t('tires.tireAddModal.createTheTires') }}
      </button>
    </template>
  </ModalShell>
</template>