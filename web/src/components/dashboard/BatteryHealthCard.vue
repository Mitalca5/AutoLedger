<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { computed, ref, watch } from 'vue'
import { intlLocale, t } from '@/i18n'
import { formatNumber } from '@/utils/numbers'
import { api } from '@/services/api'
import { BatteryCharging, ChevronDown, Trash2 } from 'lucide-vue-next'

// The battery health of an electric vehicle: the level as a gauge coloured by value, its history as a line,
// and the readings it is computed from (kept behind a fold, they are entered rarely).
const props = defineProps<{ vehicleId: string }>()
const emit = defineEmits<{ updated: [] }>()

interface Reading { date: string; health_percent?: number; current_capacity_kwh?: number; max_capacity_kwh?: number }
interface Health { readings: Reading[]; health_percent?: number; source?: string }

const health = ref<Health | null>(null)
const error = ref('')
const readingDate = ref(new Date().toISOString().slice(0, 10))
const readingPercent = ref('')
const readingMax = ref('')

async function loadHealth() {
  try {
    health.value = await api.getBatteryHealth(props.vehicleId)
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
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
    emit('updated')
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function removeReading(date: string) {
  await api.deleteBatteryReading(props.vehicleId, date)
  await loadHealth()
  emit('updated')
}

watch(() => props.vehicleId, () => {
  health.value = null
  loadHealth()
}, { immediate: true })

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
  return { line, area: `${line} L200,44 L0,44 Z` }
})
const shortDate = (iso: string) => new Date(`${iso}T00:00:00`).toLocaleDateString(intlLocale(), { day: 'numeric', month: 'short', year: 'numeric' })
const sourceLabel = computed(() => (health.value?.source ? t(`dashboard.batteryHealth.source.${health.value.source}`) : ''))
</script>

<template>
  <section class="space-y-4 rounded-2xl border border-slate-800 bg-slate-900 p-5 text-xs shadow-sm" aria-labelledby="battery-health-title">
    <h3 id="battery-health-title" class="flex items-center gap-2 text-sm font-bold text-white">
      <BatteryCharging class="h-4 w-4 text-success-400" aria-hidden="true" />
      {{ $t('dashboard.batteryHealth.title') }}
    </h3>

    <template v-if="healthPercent !== undefined">
      <div class="flex flex-wrap items-baseline gap-x-2">
        <span class="text-3xl font-extrabold" :class="healthTone.text">{{ formatNumber(healthPercent, 1) }} %</span>
        <span class="text-slate-400">{{ sourceLabel }}</span>
      </div>
      <div class="h-2 overflow-hidden rounded-full bg-slate-800" role="meter" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(healthPercent)" :aria-label="$t('dashboard.batteryHealth.title')">
        <div class="h-full rounded-full transition-all" :class="healthTone.bar" :style="{ width: `${Math.min(100, Math.max(0, healthPercent))}%` }"></div>
      </div>
    </template>
    <p v-else class="text-slate-400">{{ $t('dashboard.batteryHealth.noHealth') }}</p>

    <div v-if="historyLine">
      <svg viewBox="0 0 200 44" preserveAspectRatio="none" class="h-16 w-full" role="img" :aria-label="$t('dashboard.batteryHealth.history')">
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
        {{ $t('dashboard.batteryHealth.manageReadings', { count: health?.readings.length ?? 0 }) }}
        <ChevronDown class="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden="true" />
      </summary>
      <p class="mt-2 text-slate-400">{{ $t('dashboard.batteryHealth.healthHint') }}</p>
      <form class="mt-2 flex flex-wrap items-end gap-2" @submit.prevent="saveReading">
        <input v-model="readingDate" type="date" :aria-label="$t('dashboard.batteryHealth.date')" class="w-auto field" />
        <NumberInput text v-model="readingPercent" min="1" max="100" :placeholder="$t('dashboard.batteryHealth.percent')" :aria-label="$t('dashboard.batteryHealth.percent')" class="w-28 field" />
        <NumberInput text v-model="readingMax" min="1" :placeholder="$t('dashboard.batteryHealth.newCapacity')" :aria-label="$t('dashboard.batteryHealth.newCapacity')" class="w-36 field" />
        <button type="submit" class="btn btn-secondary" :disabled="!readingPercent && !readingMax" :title="!readingPercent && !readingMax ? $t('dashboard.batteryHealth.readingRequired') : undefined">{{ $t('dashboard.batteryHealth.addReading') }}</button>
      </form>
      <p v-if="error" class="mt-2 text-danger-400" role="alert">{{ error }}</p>
      <ul v-if="health?.readings.length" class="mt-2 space-y-1">
        <li v-for="r in [...health.readings].reverse().slice(0, 6)" :key="r.date" class="flex items-center justify-between text-slate-300">
          <span>{{ r.date }} · <template v-if="r.health_percent !== undefined">{{ r.health_percent }} %</template><template v-else-if="r.current_capacity_kwh !== undefined">{{ r.current_capacity_kwh }} kWh</template><template v-else>{{ r.max_capacity_kwh }} kWh</template></span>
          <button type="button" class="text-slate-400 hover:text-danger-400" :aria-label="$t('dashboard.batteryHealth.deleteReading', { date: r.date })" @click="removeReading(r.date)"><Trash2 class="w-4 h-4" /></button>
        </li>
      </ul>
    </details>
  </section>
</template>
