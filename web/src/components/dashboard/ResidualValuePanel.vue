<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { Trash2 } from 'lucide-vue-next'

const props = defineProps<{ vehicleId: string; showBattery: boolean }>()
const vehicleStore = useVehicleStore()

interface Reading { date: string; health_percent?: number; current_capacity_kwh?: number; max_capacity_kwh?: number }
interface Health { readings: Reading[]; health_percent?: number; source?: string; reference_kwh?: number; estimated_capacity_kwh?: number }
interface Residual {
  current_value: number; depreciation: number; progress: number; health_factor: number; health_percent?: number
  holding_months: number; age_months: number; distance_km: number; purchase_price: number; expected_resale_value: number
  curve: { month: number; value: number }[]
}

const health = ref<Health | null>(null)
const residual = ref<Residual | null>(null)
const error = ref('')
const residualError = ref('')
const readingDate = ref(new Date().toISOString().slice(0, 10))
const readingPercent = ref('')
const readingMax = ref('')
const expectedKm = ref('')
const kmShare = ref('50')
const healthWeight = ref('1')

async function loadHealth() {
  try {
    health.value = await api.getBatteryHealth(props.vehicleId)
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function loadResidual() {
  residualError.value = ''
  const km = Number(expectedKm.value)
  try {
    residual.value = await api.getResidualValue(props.vehicleId, {
      expected_km: km > 0 ? km : undefined,
      km_share: Number(kmShare.value) / 100,
      health_weight: Number(healthWeight.value),
    })
  } catch (e) {
    residual.value = null
    residualError.value = e instanceof Error ? e.message : ''
  }
}

async function saveReading() {
  error.value = ''
  try {
    health.value = await api.saveBatteryReading(props.vehicleId, {
      date: readingDate.value,
      health_percent: readingPercent.value ? Number(readingPercent.value) : undefined,
      max_capacity_kwh: readingMax.value ? Number(readingMax.value) : undefined,
    })
    readingPercent.value = ''
    await loadResidual()
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function removeReading(date: string) {
  await api.deleteBatteryReading(props.vehicleId, date)
  await loadHealth()
  await loadResidual()
}

watch(() => props.vehicleId, () => {
  health.value = null
  residual.value = null
  if (props.showBattery) loadHealth()
  loadResidual()
}, { immediate: true })

const curvePath = computed(() => {
  const curve = residual.value?.curve ?? []
  if (curve.length < 2) return ''
  const max = Math.max(...curve.map((p) => p.value))
  const min = Math.min(...curve.map((p) => p.value))
  const span = max - min || 1
  return curve.map((p, i) => `${i ? 'L' : 'M'}${(p.month / curve[curve.length - 1].month) * 200},${40 - ((p.value - min) / span) * 36 - 2}`).join(' ')
})
const marker = computed(() => {
  const r = residual.value
  if (!r || !r.curve.length) return null
  const max = Math.max(...r.curve.map((p) => p.value))
  const min = Math.min(...r.curve.map((p) => p.value))
  const x = Math.min(Math.max(r.progress, 0), 1) * 200
  return { x, y: 40 - ((r.current_value - min) / (max - min || 1)) * 36 - 2 }
})
const sourceLabel = computed(() => (health.value?.source ? t(`dashboard.residualPanel.source.${health.value.source}`) : ''))
</script>

<template>
  <details class="p-4 bg-slate-900 border border-slate-800 rounded-2xl text-xs" open>
    <summary class="font-bold text-white cursor-pointer">{{ $t('dashboard.residualPanel.title') }}</summary>
    <div class="mt-3 grid grid-cols-1 lg:grid-cols-2 gap-6">
      <div v-if="showBattery" class="space-y-3">
        <h4 class="font-bold text-slate-200">{{ $t('dashboard.residualPanel.health') }}</h4>
        <p v-if="health?.health_percent !== undefined" class="text-xl font-bold text-white">
          {{ health.health_percent }} <span class="text-xs font-medium text-slate-400">% · {{ sourceLabel }}</span>
        </p>
        <p v-else class="text-slate-400">{{ $t('dashboard.residualPanel.noHealth') }}</p>
        <p class="text-slate-400">{{ $t('dashboard.residualPanel.healthHint') }}</p>
        <form class="flex flex-wrap gap-2 items-end" @submit.prevent="saveReading">
          <input v-model="readingDate" type="date" :aria-label="$t('dashboard.residualPanel.date')" class="bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" />
          <input v-model="readingPercent" type="number" min="1" max="100" step="0.1" :placeholder="$t('dashboard.residualPanel.percent')" :aria-label="$t('dashboard.residualPanel.percent')" class="w-28 bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" />
          <input v-model="readingMax" type="number" min="1" step="0.1" :placeholder="$t('dashboard.residualPanel.newCapacity')" :aria-label="$t('dashboard.residualPanel.newCapacity')" class="w-36 bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" />
          <button type="submit" class="px-3 py-2 rounded-xl bg-cyan-500/20 text-cyan-200 border border-cyan-500/30 font-semibold disabled:opacity-50" :disabled="!readingPercent && !readingMax">{{ $t('dashboard.residualPanel.addReading') }}</button>
        </form>
        <p v-if="error" class="text-danger-400" role="alert">{{ error }}</p>
        <ul v-if="health?.readings.length" class="space-y-1">
          <li v-for="r in [...health.readings].reverse().slice(0, 6)" :key="r.date" class="flex items-center justify-between text-slate-300">
            <span>{{ r.date }} · <template v-if="r.health_percent !== undefined">{{ r.health_percent }} %</template><template v-else-if="r.current_capacity_kwh !== undefined">{{ r.current_capacity_kwh }} kWh</template><template v-else>{{ r.max_capacity_kwh }} kWh</template></span>
            <button type="button" class="text-slate-400 hover:text-danger-400" :aria-label="$t('dashboard.residualPanel.deleteReading', { date: r.date })" @click="removeReading(r.date)"><Trash2 class="w-4 h-4" /></button>
          </li>
        </ul>
      </div>

      <div class="space-y-3">
        <h4 class="font-bold text-slate-200">{{ $t('dashboard.residualPanel.residual') }}</h4>
        <div class="flex flex-wrap gap-2 items-end">
          <label class="text-slate-400">{{ $t('dashboard.residualPanel.expectedKm') }}
            <input v-model="expectedKm" type="number" min="0" step="1000" class="block w-32 bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" @change="loadResidual" />
          </label>
          <label class="text-slate-400">{{ $t('dashboard.residualPanel.kmShare') }}
            <input v-model="kmShare" type="number" min="0" max="100" step="5" class="block w-24 bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" @change="loadResidual" />
          </label>
          <label v-if="showBattery" class="text-slate-400">{{ $t('dashboard.residualPanel.healthWeight') }}
            <input v-model="healthWeight" type="number" min="0" max="2" step="0.1" class="block w-24 bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" @change="loadResidual" />
          </label>
        </div>
        <template v-if="residual">
          <p class="text-xl font-bold text-white">{{ formatAmount(residual.current_value, vehicleStore.currency) }}</p>
          <p class="text-slate-400">
            {{ $t('dashboard.residualPanel.summary', { depreciation: formatAmount(residual.depreciation, vehicleStore.currency), progress: Math.round(residual.progress * 100) }) }}
          </p>
          <svg v-if="curvePath" viewBox="0 0 200 40" class="w-full h-16" role="img" :aria-label="$t('dashboard.residualPanel.curve')">
            <path :d="curvePath" fill="none" stroke="#a78bfa" stroke-width="1.5" />
            <circle v-if="marker" :cx="marker.x" :cy="marker.y" r="3" fill="#34d399" />
          </svg>
          <p class="text-slate-400">{{ $t('dashboard.residualPanel.curveHint', { months: residual.holding_months, value: formatAmount(residual.expected_resale_value, vehicleStore.currency) }) }}</p>
        </template>
        <p v-else-if="residualError" class="text-slate-400" role="status">{{ residualError }}</p>
      </div>
    </div>
  </details>
</template>
