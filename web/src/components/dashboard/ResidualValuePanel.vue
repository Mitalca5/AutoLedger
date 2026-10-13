<script setup lang="ts">
import DistanceInput from '@/components/DistanceInput.vue'
import NumberInput from '@/components/NumberInput.vue'
import { computed, ref, watch } from 'vue'
import { intlLocale, t } from '@/i18n'
import { formatNumber } from '@/utils/numbers'
import { api } from '@/services/api'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { BatteryCharging, ChevronDown, Trash2 } from 'lucide-vue-next'

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
// Collapsed until there is something to show: a fresh vehicle only sees a "Configure" prompt, not a form of empty fields.
const hasContent = computed(() => health.value?.health_percent !== undefined || !!health.value?.readings?.length || !!residual.value)
const isOpen = ref(false)
const userToggled = ref(false)
watch(hasContent, (filled) => {
  if (!userToggled.value) isOpen.value = filled
})
function onToggle(e: Event) {
  const open = (e.target as HTMLDetailsElement).open
  if (open !== isOpen.value) {
    userToggled.value = true
    isOpen.value = open
  }
}
// The battery health as a gauge, coloured by level, and the readings that carry a percentage as a history line
const healthPercent = computed(() => health.value?.health_percent)
const healthTone = computed(() => {
  const v = healthPercent.value ?? 0
  if (v >= 90) return { text: 'text-success-400', bar: 'bg-success-500' }
  if (v >= 80) return { text: 'text-warning-400', bar: 'bg-warning-500' }
  return { text: 'text-danger-400', bar: 'bg-danger-500' }
})
const history = computed(() =>
  (health.value?.readings ?? [])
    .filter((r) => r.health_percent !== undefined)
    .slice(-60)
    .map((r) => ({ date: r.date, value: r.health_percent as number })),
)
const historyLine = computed(() => {
  const pts = history.value
  if (pts.length < 2) return null
  const values = pts.map((p) => p.value)
  const max = Math.max(...values)
  const min = Math.min(...values)
  const span = max - min || 1
  const xy = pts.map((p, i) => [(i / (pts.length - 1)) * 200, 40 - ((p.value - min) / span) * 32 - 4] as const)
  const line = xy.map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
  return { line, area: `${line} L200,44 L0,44 Z`, last: xy[xy.length - 1] }
})
const shortDate = (iso: string) => new Date(`${iso}T00:00:00`).toLocaleDateString(intlLocale(), { day: 'numeric', month: 'short', year: 'numeric' })
const sourceLabel = computed(() => (health.value?.source ? t(`dashboard.residualPanel.source.${health.value.source}`) : ''))
</script>

<template>
  <details class="rounded-2xl border border-slate-800 bg-slate-900 p-5 text-xs shadow-sm" :open="isOpen" @toggle="onToggle">
    <summary class="flex cursor-pointer items-center justify-between gap-2 font-bold text-white">
      <h3 class="flex items-center gap-2 text-sm font-bold text-white">
        <BatteryCharging v-if="showBattery" class="h-4 w-4 text-success-400" aria-hidden="true" />
        {{ $t('dashboard.residualPanel.title') }}
      </h3>
      <span v-if="!isOpen" class="rounded-full border border-slate-700 bg-slate-800 px-2.5 py-0.5 text-[11px] font-semibold text-slate-300">{{ hasContent ? $t('dashboard.residualPanel.show') : $t('dashboard.residualPanel.configure') }}</span>
    </summary>
    <div class="mt-4 grid grid-cols-1 gap-6 lg:grid-cols-2">
      <div v-if="showBattery" class="space-y-3">
        <h4 class="font-bold text-slate-200">{{ $t('dashboard.residualPanel.health') }}</h4>
        <template v-if="healthPercent !== undefined">
          <div class="flex flex-wrap items-baseline gap-x-2">
            <span class="text-2xl font-extrabold" :class="healthTone.text">{{ formatNumber(healthPercent, 1) }} %</span>
            <span class="text-xs text-slate-400">{{ sourceLabel }}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-slate-800" role="meter" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(healthPercent)" :aria-label="$t('dashboard.residualPanel.health')">
            <div class="h-full rounded-full transition-all" :class="healthTone.bar" :style="{ width: `${Math.min(100, Math.max(0, healthPercent))}%` }"></div>
          </div>
        </template>
        <p v-else class="text-slate-400">{{ $t('dashboard.residualPanel.noHealth') }}</p>
        <div v-if="historyLine">
          <svg viewBox="0 0 200 44" preserveAspectRatio="none" class="h-16 w-full" role="img" :aria-label="$t('dashboard.residualPanel.history')">
            <path :d="historyLine.area" fill="#34d399" fill-opacity="0.12" />
            <path :d="historyLine.line" fill="none" stroke="#34d399" stroke-width="1.5" vector-effect="non-scaling-stroke" />
          </svg>
          <p class="flex justify-between text-slate-400">
            <span>{{ shortDate(history[0].date) }} · {{ formatNumber(history[0].value, 1) }} %</span>
            <span>{{ shortDate(history[history.length - 1].date) }} · {{ formatNumber(history[history.length - 1].value, 1) }} %</span>
          </p>
        </div>
        <details class="group rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2">
          <summary class="flex cursor-pointer list-none items-center justify-between gap-2 font-semibold text-slate-300">
            {{ $t('dashboard.residualPanel.manageReadings', { count: health?.readings.length ?? 0 }) }}
            <ChevronDown class="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden="true" />
          </summary>
          <p class="mt-2 text-slate-400">{{ $t('dashboard.residualPanel.healthHint') }}</p>
          <form class="mt-2 flex flex-wrap items-end gap-2" @submit.prevent="saveReading">
            <input v-model="readingDate" type="date" :aria-label="$t('dashboard.residualPanel.date')" class="w-auto field" />
            <NumberInput text v-model="readingPercent" min="1" max="100" :placeholder="$t('dashboard.residualPanel.percent')" :aria-label="$t('dashboard.residualPanel.percent')" class="w-28 field" />
            <NumberInput text v-model="readingMax" min="1" :placeholder="$t('dashboard.residualPanel.newCapacity')" :aria-label="$t('dashboard.residualPanel.newCapacity')" class="w-36 field" />
            <button type="submit" class="px-3 py-2 rounded-xl bg-sky-500/20 text-sky-200 border border-sky-500/30 font-semibold disabled:opacity-50" :disabled="!readingPercent && !readingMax" :title="!readingPercent && !readingMax ? $t('dashboard.residualPanel.readingRequired') : undefined">{{ $t('dashboard.residualPanel.addReading') }}</button>
          </form>
          <p v-if="error" class="mt-2 text-danger-400" role="alert">{{ error }}</p>
          <ul v-if="health?.readings.length" class="mt-2 space-y-1">
            <li v-for="r in [...health.readings].reverse().slice(0, 6)" :key="r.date" class="flex items-center justify-between text-slate-300">
              <span>{{ r.date }} · <template v-if="r.health_percent !== undefined">{{ r.health_percent }} %</template><template v-else-if="r.current_capacity_kwh !== undefined">{{ r.current_capacity_kwh }} kWh</template><template v-else>{{ r.max_capacity_kwh }} kWh</template></span>
              <button type="button" class="text-slate-400 hover:text-danger-400" :aria-label="$t('dashboard.residualPanel.deleteReading', { date: r.date })" @click="removeReading(r.date)"><Trash2 class="w-4 h-4" /></button>
            </li>
          </ul>
        </details>
      </div>

      <div class="space-y-3">
        <h4 class="font-bold text-slate-200">{{ $t('dashboard.residualPanel.residual') }}</h4>
        <div class="flex flex-wrap gap-2 items-end">
          <label class="text-slate-400">{{ $t('dashboard.residualPanel.expectedKm') }}
            <DistanceInput whole text v-model="expectedKm" min="0" step="1000" class="block w-32 field" @change="loadResidual" />
          </label>
        </div>
        <details class="group rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2">
          <summary class="flex cursor-pointer list-none items-center justify-between gap-2 font-semibold text-slate-300">
            {{ $t('dashboard.residualPanel.advanced') }}
            <ChevronDown class="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden="true" />
          </summary>
          <p class="mt-2 text-slate-400">{{ $t('dashboard.residualPanel.advancedHint') }}</p>
          <div class="mt-2 flex flex-wrap items-end gap-2">
            <label class="text-slate-400">{{ $t('dashboard.residualPanel.kmShare') }}
              <input v-model="kmShare" type="number" min="0" max="100" step="5" class="block w-24 field" @change="loadResidual" />
            </label>
            <label v-if="showBattery" for="residual-health-weight" class="text-slate-400">{{ $t('dashboard.residualPanel.healthWeight') }}
              <NumberInput id="residual-health-weight" text v-model="healthWeight" min="0" max="2" class="block w-24 field" @change="loadResidual" />
            </label>
          </div>
        </details>
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
