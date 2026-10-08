<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { ref, computed, watch, onMounted } from 'vue'
import { Calculator, X, Sparkles, Check, BookmarkPlus, Info } from 'lucide-vue-next'
import { api, type PublicChargingPreset, type PublicChargingBreakdown } from '@/services/api'
import { formatAmount } from '@/currency'
import { t } from '@/i18n'
import { publicChargingRates, publicChargingRequest } from '@/utils/publicCharging'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

const props = defineProps<{
  currency: string
  initialKwh?: number | null
}>()

const emit = defineEmits<{
  apply: [cost: number, kwh?: number, notes?: string]
}>()

const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))

const presets = ref<PublicChargingPreset[]>([])
const selectedPresetId = ref<string>('')

const kwh = ref<number | ''>(props.initialKwh || '')
const chargingMinutes = ref<number | ''>('')
const idleMinutes = ref<number | ''>('')

const connectionFee = ref<number | ''>('')
const costPerKwh = ref<number | ''>('')
const costPerMinute = ref<number | ''>('')
const idleFeePerMinute = ref<number | ''>('')
const idleGraceMinutes = ref<number | ''>('')

const breakdown = ref<PublicChargingBreakdown | null>(null)
const calculating = ref(false)
let latestCalculation = 0

const savePresetName = ref('')
const showSavePreset = ref(false)
const savingPreset = ref(false)
const rates = () => ({
  connectionFee: connectionFee.value,
  costPerKwh: costPerKwh.value,
  costPerMinute: costPerMinute.value,
  idleFeePerMinute: idleFeePerMinute.value,
  idleGraceMinutes: idleGraceMinutes.value,
})

async function loadPresets() {
  try {
    const res = await api.getPublicPresets()
    presets.value = res.presets || []
  } catch (err) {
    console.error('Failed to load public presets', err)
  }
}

watch(selectedPresetId, (presetId) => {
  if (!presetId) return
  const preset = presets.value.find((p) => p.id === presetId)
  if (!preset) return

  connectionFee.value = preset.connection_fee ?? ''
  costPerKwh.value = preset.price_per_kwh ?? ''
  costPerMinute.value = preset.price_per_minute ?? ''
  idleFeePerMinute.value = preset.idle_fee_per_minute ?? ''
  idleGraceMinutes.value = preset.idle_grace_minutes ?? ''
  calculate()
})

watch([kwh, chargingMinutes, idleMinutes, connectionFee, costPerKwh, costPerMinute, idleFeePerMinute, idleGraceMinutes], () => {
  calculate()
})

watch(open, (isOpen) => {
  if (isOpen) {
    if (props.initialKwh && !kwh.value) {
      kwh.value = props.initialKwh
    }
    loadPresets()
    calculate()
  }
})

async function calculate() {
  const seq = ++latestCalculation
  breakdown.value = null
  const request = publicChargingRequest(rates(), kwh.value, chargingMinutes.value, idleMinutes.value)
  if (request.kwh <= 0 && request.total_plugged_minutes <= 0 && !request.connection_fee) {
    calculating.value = false
    return
  }

  calculating.value = true
  try {
    const res = await api.calculatePublicCharge(request)
    if (seq === latestCalculation) breakdown.value = res
  } catch (err) {
    console.error('Failed to calculate public charge', err)
  } finally {
    if (seq === latestCalculation) calculating.value = false
  }
}

async function handleSavePreset() {
  if (!savePresetName.value.trim()) return
  savingPreset.value = true
  try {
    const newPreset = await api.createPublicPreset({
      name: savePresetName.value.trim(),
      currency: props.currency,
      ...publicChargingRates(rates()),
    })
    presets.value.push(newPreset)
    selectedPresetId.value = newPreset.id
    savePresetName.value = ''
    showSavePreset.value = false
  } catch (err) {
    console.error('Failed to save public charging preset', err)
  } finally {
    savingPreset.value = false
  }
}

function applyToCharge() {
  if (!breakdown.value || calculating.value || !Number.isFinite(breakdown.value.total_cost) || breakdown.value.total_cost < 0) return
  const totalCost = breakdown.value.total_cost
  const k = Number(kwh.value) || undefined
  emit('apply', totalCost, k)
  open.value = false
}

onMounted(() => {
  loadPresets()
})
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-modal-nested flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm" @click.self="open = false">
    <div v-dialog class="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
      <!-- Header -->
      <div class="flex items-center justify-between p-4 border-b border-slate-800">
        <div class="flex items-center gap-2">
          <div class="p-2 bg-blue-500/10 text-blue-400 rounded-lg">
            <Calculator class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-bold text-white">{{ t('tariffs.publicModal.title') }}</h2>
            <p class="text-xs text-slate-400">{{ t('tariffs.publicModal.subtitle') }}</p>
          </div>
        </div>
        <button @click="open = false" class="tap p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800" :aria-label="$t('common.close')">
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Content -->
      <div class="p-5 overflow-y-auto space-y-4 text-xs">
        <!-- Preset selector -->
        <div>
          <label for="public-preset-select" class="block text-slate-300 font-medium mb-1">{{ t('tariffs.publicModal.presetLabel') }}</label>
          <select
            id="public-preset-select"
            v-model="selectedPresetId"
            class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-white focus:border-blue-500 focus:outline-none"
          >
            <option value="">{{ t('tariffs.publicModal.customPricing') }}</option>
            <option v-for="p in presets" :key="p.id" :value="p.id">
              {{ p.name }}
            </option>
          </select>
        </div>

        <!-- Session Consumption Inputs -->
        <div class="grid grid-cols-3 gap-3 p-3 bg-slate-950/60 border border-slate-800 rounded-xl">
          <div>
            <label for="public-charge-kwh" class="block text-slate-400 mb-1 font-medium">{{ t('tariffs.publicModal.energyKwh') }}</label>
            <NumberInput text
              id="public-charge-kwh"
              v-model="kwh"
              min="0"
              placeholder="0.00"
              class="field focus:border-blue-500"
            />
          </div>
          <div>
            <label for="public-charge-duration" class="block text-slate-400 mb-1 font-medium">{{ t('tariffs.publicModal.durationMin') }}</label>
            <input
              id="public-charge-duration"
              v-model="chargingMinutes"
              type="number"
              step="1"
              min="0"
              placeholder="0"
              class="field focus:border-blue-500"
            />
          </div>
          <div>
            <label for="public-charge-idle" class="block text-slate-400 mb-1 font-medium">{{ t('tariffs.publicModal.idleMin') }}</label>
            <input
              id="public-charge-idle"
              v-model="idleMinutes"
              type="number"
              step="1"
              min="0"
              placeholder="0"
              class="field focus:border-blue-500"
            />
          </div>
        </div>

        <!-- Pricing Rates -->
        <div class="space-y-3">
          <span class="block font-semibold text-slate-300">{{ t('tariffs.publicModal.ratesSection') }}</span>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="public-charge-conn-fee" class="block text-slate-400 mb-1">{{ t('tariffs.publicModal.connectionFee') }} ({{ currency }})</label>
              <NumberInput text
                id="public-charge-conn-fee"
                v-model="connectionFee"
                min="0"
                placeholder="0.00"
                class="w-full rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-white focus:border-blue-500 focus:outline-none"
              />
            </div>
            <div>
              <label for="public-charge-kwh-cost" class="block text-slate-400 mb-1">{{ t('tariffs.publicModal.costPerKwh') }} ({{ currency }}/kWh)</label>
              <NumberInput text
                id="public-charge-kwh-cost"
                v-model="costPerKwh"
                min="0"
                placeholder="0.00"
                class="w-full rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-white focus:border-blue-500 focus:outline-none"
              />
            </div>
            <div>
              <label for="public-charge-min-cost" class="block text-slate-400 mb-1">{{ t('tariffs.publicModal.costPerMin') }} ({{ currency }}/min)</label>
              <NumberInput text
                id="public-charge-min-cost"
                v-model="costPerMinute"
                min="0"
                placeholder="0.00"
                class="w-full rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-white focus:border-blue-500 focus:outline-none"
              />
            </div>
            <div>
              <label for="public-charge-idle-cost" class="block text-slate-400 mb-1">{{ t('tariffs.publicModal.idleFeePerMin') }} ({{ currency }}/min)</label>
              <NumberInput text
                id="public-charge-idle-cost"
                v-model="idleFeePerMinute"
                min="0"
                placeholder="0.00"
                class="w-full rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-white focus:border-blue-500 focus:outline-none"
              />
            </div>
          </div>
          <div>
            <label for="public-charge-grace" class="block text-slate-400 mb-1">{{ t('tariffs.publicModal.idleGraceMin') }}</label>
            <input
              id="public-charge-grace"
              v-model="idleGraceMinutes"
              type="number"
              step="1"
              min="0"
              placeholder="ex: 5 min"
              class="w-1/2 rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-white focus:border-blue-500 focus:outline-none"
            />
          </div>
        </div>

        <!-- Save Preset Toggle -->
        <div class="pt-2 border-t border-slate-800">
          <div v-if="!showSavePreset">
            <button
              type="button"
              @click="showSavePreset = true"
              class="inline-flex items-center gap-1.5 text-blue-400 hover:text-blue-300 font-medium"
            >
              <BookmarkPlus class="w-3.5 h-3.5" />
              {{ t('tariffs.publicModal.saveAsPreset') }}
            </button>
          </div>
          <div v-else class="flex items-center gap-2">
            <label for="public-charge-preset-name" class="sr-only">{{ t('tariffs.publicModal.saveAsPreset') }}</label>
            <input
              id="public-charge-preset-name"
              v-model="savePresetName"
              type="text"
              placeholder="ex: Ionity Direct"
              class="flex-1 rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-white focus:border-blue-500 focus:outline-none"
            />
            <button
              type="button"
              :disabled="savingPreset || !savePresetName.trim()"
              @click="handleSavePreset"
              class="px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-semibold disabled:opacity-50"
            >
              {{ t('common.save') }}
            </button>
            <button
              type="button"
              @click="showSavePreset = false"
              class="px-2 py-1.5 text-slate-400 hover:text-white"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Breakdown Result -->
        <div v-if="breakdown" class="p-3.5 bg-blue-950/20 border border-blue-500/30 rounded-xl space-y-2">
          <div class="flex items-center justify-between">
            <span class="font-bold text-white text-sm">{{ t('tariffs.publicModal.estimatedTotal') }}</span>
            <span class="font-extrabold text-base text-blue-400">
              {{ formatAmount(breakdown.total_cost, currency) }}
            </span>
          </div>

          <div class="space-y-1 pt-2 border-t border-blue-500/20 text-xs text-slate-300">
            <div v-if="breakdown.connection_cost > 0" class="flex justify-between">
              <span>{{ t('tariffs.publicModal.breakdownConnection') }}</span>
              <span>{{ formatAmount(breakdown.connection_cost, currency) }}</span>
            </div>
            <div v-if="breakdown.energy_cost > 0" class="flex justify-between">
              <span>{{ t('tariffs.publicModal.breakdownEnergy') }}</span>
              <span>{{ formatAmount(breakdown.energy_cost, currency) }}</span>
            </div>
            <div v-if="breakdown.duration_cost > 0" class="flex justify-between">
              <span>{{ t('tariffs.publicModal.breakdownDuration') }}</span>
              <span>{{ formatAmount(breakdown.duration_cost, currency) }}</span>
            </div>
            <div v-if="breakdown.idle_cost > 0" class="flex justify-between text-warning-300">
              <span>{{ t('tariffs.publicModal.breakdownIdle') }}</span>
              <span>{{ formatAmount(breakdown.idle_cost, currency) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-end gap-2 p-4 border-t border-slate-800 bg-slate-950/40">
        <button
          type="button"
          @click="open = false"
          class="btn btn-lg btn-secondary"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          :disabled="calculating || !breakdown || !Number.isFinite(breakdown.total_cost) || breakdown.total_cost < 0"
          @click="applyToCharge"
          class="btn btn-lg btn-primary"
        >
          {{ t('tariffs.publicModal.applyButton') }}
        </button>
      </div>
    </div>
  </div>
</template>
