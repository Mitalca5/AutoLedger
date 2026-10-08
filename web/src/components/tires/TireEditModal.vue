<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { Pencil, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { isMountedPosition } from '@/utils/tires'
import { toIsoDay } from '@/utils/dates'
import { useVehicleStore } from '@/stores/vehicle'
import { currencySymbol } from '@/currency'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { useOdometerPrefill } from '@/composables/useOdometerPrefill'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

// Empty fields are left unchanged. fallbackTire / fallbackStats stand in for a tire the list does not carry (opened from its history).
const props = defineProps<{ vehicleId: string; tires: any[]; tireIds: string[]; fallbackTire: any | null; fallbackStats: any | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const vehicleStore = useVehicleStore()

const tireEditIds = ref<string[]>([])
const tireEditPriceMode = ref<'UNIT' | 'TOTAL'>('UNIT')
const tireEditForm = ref(emptyTireEdit())

function emptyTireEdit() {
  return {
    brand: '',
    model: '',
    dimension: '',
    season: '',
    purchase_date: '',
    price: '' as number | string,
    dot_code: '',
    initial_depth_mm: '' as number | string,
    min_legal_depth_mm: '' as number | string,
    initial_distance_km: '' as number | string,
    estimated_lifespan_km: '' as number | string,
    mounted_date: '',
    mounted_odometer: '' as number | string,
  }
}

const editedTires = computed(() => props.tires.filter((t) => tireEditIds.value.includes(t.tire.id)))
const editIncludesMounted = computed(() => editedTires.value.some((t) => isMountedPosition(t.tire.current_position)))

// A single tire's fitting odometer is prefilled from the fitting date while it is empty; with several tires an
// empty field means "unchanged" and stays so.
const mountedPrefill = useOdometerPrefill({
  vehicleId: () => props.vehicleId,
  enabled: () => open.value && tireEditIds.value.length === 1 && editIncludesMounted.value,
  date: () => tireEditForm.value.mounted_date,
  current: () => tireEditForm.value.mounted_odometer,
  fill: (km) => {
    tireEditForm.value.mounted_odometer = km
  },
})

watch(open, (isOpen) => {
  if (isOpen) {
    init([...props.tireIds])
    mountedPrefill.reset(Number(tireEditForm.value.mounted_odometer) || null)
  }
})

function init(ids: string[]) {
  tireEditIds.value = ids
  tireEditPriceMode.value = ids.length > 1 ? 'TOTAL' : 'UNIT'
  const form = emptyTireEdit()
  if (ids.length === 1) {
    const stats = props.tires.find((t) => t.tire.id === ids[0]) || props.fallbackStats
    const t = stats?.tire || props.fallbackTire
    const activeSession = (stats?.sessions || []).find((ss: any) => !ss.dismounted_date)
    Object.assign(form, {
      brand: t.brand,
      model: t.model,
      dimension: t.dimension,
      season: t.season,
      purchase_date: t.purchase_date ? toIsoDay(t.purchase_date) : '',
      price: t.purchase_price,
      dot_code: t.dot_code || '',
      initial_depth_mm: t.initial_depth_mm,
      min_legal_depth_mm: t.min_legal_depth_mm,
      initial_distance_km: t.initial_distance_km,
      estimated_lifespan_km: t.estimated_lifespan_km,
      mounted_date: activeSession ? toIsoDay(activeSession.mounted_date) : '',
      mounted_odometer: activeSession ? activeSession.mounted_odometer : '',
    })
  }
  tireEditForm.value = form
}

const { pending: submitting, run: runOnce } = useSubmit()

async function handleSaveTireEditAction() {
  if (!props.vehicleId || !tireEditIds.value.length) return
  const f = tireEditForm.value
  const text = (v: string) => (v.trim() ? v.trim() : undefined)
  const num = (v: number | string) => (v === '' || v === null ? undefined : Number(v))
  const payload: any = {
    tire_ids: tireEditIds.value,
    brand: text(f.brand),
    model: text(f.model),
    dimension: text(f.dimension),
    season: f.season || undefined,
    purchase_date: f.purchase_date || undefined,
    dot_code: text(f.dot_code),
    initial_depth_mm: num(f.initial_depth_mm),
    min_legal_depth_mm: num(f.min_legal_depth_mm),
    initial_distance_km: num(f.initial_distance_km),
    estimated_lifespan_km: num(f.estimated_lifespan_km),
    mounted_date: f.mounted_date ? new Date(f.mounted_date).toISOString() : undefined,
    mounted_odometer: num(f.mounted_odometer),
  }
  if (num(f.price) !== undefined) {
    payload[tireEditPriceMode.value === 'TOTAL' ? 'total_price' : 'purchase_price'] = num(f.price)
  }
  try {
    await api.batchUpdateTires(props.vehicleId, payload)
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleSaveTireEdit = () => runOnce(handleSaveTireEditAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-modal bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Pencil class="w-4 h-4 text-rose-400" />
          {{ tireEditIds.length > 1 ? $t('tires.tireEditModal.editMany', { count: tireEditIds.length }) : $t('tires.tireEditModal.editOne') }}
        </h3>
        <button type="button" @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-4 h-4" />
        </button>
      </div>

      <form id="tire-edit-modal-form" @submit.prevent="handleSaveTireEdit" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <p v-if="tireEditIds.length > 1" class="text-xs text-slate-400">
          {{ $t('tires.tireEditModal.emptyFieldsUnchanged', { tires: editedTires.map((t) => `${t.tire.brand} ${t.tire.current_position}`).join(' • ') }) }}
        </p>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label for="tire-edit-brand" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.brand') }}</label>
            <input id="tire-edit-brand" v-model="tireEditForm.brand" type="text"  :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
          <div>
            <label for="tire-edit-model" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.model') }}</label>
            <input id="tire-edit-model" v-model="tireEditForm.model" type="text"  :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
          <div>
            <label for="tire-edit-dimension" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.size') }}</label>
            <input id="tire-edit-dimension" v-model="tireEditForm.dimension" type="text"  :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
          <div>
            <label for="tire-edit-season" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.season') }}</label>
            <select id="tire-edit-season" v-model="tireEditForm.season" class="field">
              <option value="">{{ tireEditIds.length > 1 ? $t('tires.tireEditModal.unchangedFeminine') : '—' }}</option>
              <option value="SUMMER">{{ $t('tires.tireEditModal.summer') }}</option>
              <option value="WINTER">{{ $t('tires.tireEditModal.winter') }}</option>
              <option value="ALL_SEASON">{{ $t('tires.tireEditModal.allSeason') }}</option>
            </select>
          </div>
          <div>
            <label for="tire-edit-purchase-date" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.purchaseDate') }}</label>
            <AppDatePicker
              id="tire-edit-purchase-date"
              v-model="tireEditForm.purchase_date"
              :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : $t('tires.tireEditModal.selectDate')"
              size="xs"
              :clearable="true"
            />
          </div>
          <div>
            <label for="tire-edit-dot" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.dotCode') }}</label>
            <input id="tire-edit-dot" v-model="tireEditForm.dot_code" type="text"  :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3 items-end">
          <div>
            <label for="tire-edit-price-mode" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.priceEntry') }}</label>
            <select id="tire-edit-price-mode" v-model="tireEditPriceMode" class="field">
              <option value="UNIT">{{ $t('tires.tireEditModal.unitPrice') }}</option>
              <option value="TOTAL" :disabled="tireEditIds.length < 2">{{ $t('tires.tireEditModal.totalPriceSplit') }}</option>
            </select>
          </div>
          <div>
            <label for="tire-edit-price" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.price', { cur: currencySymbol(vehicleStore.currency) }) }}</label>
            <NumberInput id="tire-edit-price" v-model="tireEditForm.price" min="0" :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
          <div>
            <label for="tire-edit-lifespan" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.estimatedLifespanKm', { unit: distanceUnit() }) }}</label>
            <DistanceInput whole id="tire-edit-lifespan" v-model="tireEditForm.estimated_lifespan_km" min="1" :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
          <div>
            <label for="tire-edit-initial-depth" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.newDepthMm') }}</label>
            <NumberInput id="tire-edit-initial-depth" v-model="tireEditForm.initial_depth_mm" min="0" :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
          <div>
            <label for="tire-edit-min-depth" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.minimumDepthMm') }}</label>
            <NumberInput id="tire-edit-min-depth" v-model="tireEditForm.min_legal_depth_mm" min="0" :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
          <div>
            <label for="tire-edit-initial-distance" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.kmBeforeTrackingUsed', { unit: distanceUnit() }) }}</label>
            <DistanceInput id="tire-edit-initial-distance" v-model="tireEditForm.initial_distance_km" min="0" :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
          </div>
        </div>

        <div v-if="editIncludesMounted" class="space-y-2 pt-3 border-t border-slate-800">
          <h4 class="text-xs font-bold text-rose-400 uppercase tracking-wider">{{ $t('tires.tireEditModal.currentlyFitted') }}</h4>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label for="tire-edit-mounted-date" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.fittingDate') }}</label>
              <AppDatePicker
                id="tire-edit-mounted-date"
                v-model="tireEditForm.mounted_date"
                :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : $t('tires.tireEditModal.selectDate')"
                size="xs"
                :clearable="true"
              />
            </div>
            <div>
              <label for="tire-edit-mounted-odometer" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireEditModal.odometerAtFittingKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput id="tire-edit-mounted-odometer" v-model="tireEditForm.mounted_odometer" min="0" :placeholder="tireEditIds.length > 1 ? $t('tires.tireEditModal.unchanged') : ''" class="field" />
            </div>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting" type="submit" form="tire-edit-modal-form" class="btn btn-lg btn-primary">
          {{ $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
