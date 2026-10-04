<script setup lang="ts">
import { t } from '@/i18n'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Snowflake } from 'lucide-vue-next'
import { AXIS_TEXT, GRID_COLOR, fmt, perUnit, type EnergyStats } from './energyStats'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistance, formatDistanceValue, perDistance } from '@/units'

Chart.register(...registerables)

const props = defineProps<{ stats: EnergyStats }>()
const vehicleStore = useVehicleStore()

const canvas = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

const bins = computed(() => props.stats.temperature_bins)
const effect = computed(() => props.stats.temperature)

const binLabel = (b: { min_c: number; max_c: number }) => t('dashboard.energyTemperatureSection.binRange', { min: b.min_c, max: b.max_c })

// Cold to warm: the colour repeats what the axis says.
function binColor(minC: number) {
  if (minC < 0) return '#60a5fa'
  if (minC < 10) return '#38bdf8'
  if (minC < 20) return '#34d399'
  return '#f59e0b'
}

function draw() {
  chart?.destroy()
  chart = null
  if (!canvas.value || bins.value.length === 0) return
  chart = new Chart(canvas.value, {
    type: 'bar',
    data: {
      labels: bins.value.map(binLabel),
      datasets: [
        {
          label: t('dashboard.energyTemperatureSection.consumption'),
          data: bins.value.map((b) => perUnit(b.consumption_kwh_100km) ?? null),
          backgroundColor: bins.value.map((b) => binColor(b.min_c)),
          borderRadius: 4,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx) => `${fmt(Number(ctx.raw), 1)} kWh/100 ${distanceUnit()}`,
            afterLabel: (ctx) => {
              const b = bins.value[ctx.dataIndex]
              return t('dashboard.energyTemperatureSection.drivesAndDistance', { unit: distanceUnit(), drives: b.drives, distance: formatDistanceValue(b.distance_km) })
            },
          },
        },
      },
      scales: {
        x: { grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT } },
        y: { beginAtZero: true, grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT }, title: { display: true, text: `kWh/100 ${distanceUnit()}`, color: AXIS_TEXT } },
      },
    },
  })
}

watch(() => props.stats, async () => {
  await nextTick()
  draw()
}, { immediate: true, flush: 'post' })

onBeforeUnmount(() => chart?.destroy())
</script>

<template>
  <div v-if="bins.length > 0" class="space-y-3" role="group" aria-labelledby="energy-temperature-title">
    <h4 id="energy-temperature-title" class="flex items-center gap-2 text-xs font-bold text-slate-200">
      <Snowflake class="h-4 w-4 text-info-400" aria-hidden="true" />
      {{ $t('dashboard.energyTemperatureSection.effectOfTemperature') }}
    </h4>

    <p v-if="effect.extra_percent !== undefined" class="rounded-xl border border-info-500/20 bg-info-500/10 px-3 py-2 text-sm text-info-100">
      {{ $t('dashboard.energyTemperatureSection.below5CTheCar') }} <strong>{{ $t('dashboard.energyTemperatureSection.kwh100Km', { unit: distanceUnit(), value: fmt(perUnit(effect.cold_consumption_kwh_100km), 1) }) }}</strong>
      {{ $t('dashboard.energyTemperatureSection.againstInMildWeather15', { value: fmt(perUnit(effect.mild_consumption_kwh_100km), 1) }) }}
      <strong>+{{ fmt(effect.extra_percent, 0) }} %</strong><template v-if="effect.extra_cost_per_100km !== undefined">{{ $t('dashboard.energyTemperatureSection.aboutMorePer100Km', { unit: distanceUnit(), value: formatAmount(perDistance(effect.extra_cost_per_100km), vehicleStore.currency) }) }}</template>.
    </p>

    <div class="h-52">
      <canvas ref="canvas" role="img" :aria-label="$t('dashboard.energyTemperatureSection.averageConsumptionPer100Km', { unit: distanceUnit() })"></canvas>
    </div>
    <div class="sr-only">
      <table>
        <caption>{{ $t('dashboard.energyTemperatureSection.consumptionByOutsideTemperature') }}</caption>
        <thead><tr><th>{{ $t('dashboard.energyTemperatureSection.temperature') }}</th><th>kWh/100 {{ distanceUnit() }}</th><th>{{ $t('dashboard.energyTemperatureSection.drives') }}</th><th>{{ $t('dashboard.energyTemperatureSection.distanceKm', { unit: distanceUnit() }) }}</th></tr></thead>
        <tbody>
          <tr v-for="b in bins" :key="b.min_c">
            <td>{{ binLabel(b) }}</td><td>{{ fmt(perUnit(b.consumption_kwh_100km), 1) }}</td><td>{{ b.drives }}</td><td>{{ formatDistanceValue(b.distance_km) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-xs text-slate-400">{{ $t('dashboard.energyTemperatureSection.drivesOfAtLeast5', { min: formatDistance(5), band: formatDistance(50) }) }}</p>
  </div>
</template>
