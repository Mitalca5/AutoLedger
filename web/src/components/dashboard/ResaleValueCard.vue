<script setup lang="ts">
import DistanceInput from '@/components/DistanceInput.vue'
import NumberInput from '@/components/NumberInput.vue'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { ChevronDown, TrendingDown } from 'lucide-vue-next'

// What the vehicle is worth today and what it should fetch at the end of the holding period, from the purchase
// price, the expected resale value and the holding period of its ownership.
// Without those figures there is nothing to estimate: the card says what is missing and links to where to enter it.
const props = defineProps<{ vehicleId: string; hasBattery: boolean; refreshKey?: number }>()
const vehicleStore = useVehicleStore()

interface Residual {
  current_value: number; depreciation: number; progress: number
  holding_months: number; age_months: number; distance_km: number; purchase_price: number; expected_resale_value: number
  curve: { month: number; value: number }[]
}

const residual = ref<Residual | null>(null)
const missing = ref(false)
const expectedKm = ref('')
const kmShare = ref('50')
const healthWeight = ref('1')

async function load() {
  const km = Number(expectedKm.value)
  try {
    residual.value = await api.getResidualValue(props.vehicleId, {
      expected_km: km > 0 ? km : undefined,
      km_share: Number(kmShare.value) / 100,
      health_weight: Number(healthWeight.value),
    })
    missing.value = false
  } catch {
    residual.value = null
    missing.value = true
  }
}

watch(() => [props.vehicleId, props.refreshKey], () => {
  residual.value = null
  missing.value = false
  load()
}, { immediate: true })

const cur = computed(() => vehicleStore.currency)
const progressPct = computed(() => Math.round(Math.min(Math.max(residual.value?.progress ?? 0, 0), 1) * 100))
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
  return { x: Math.min(Math.max(r.progress, 0), 1) * 200 }
})
</script>

<template>
  <section class="space-y-4 rounded-2xl border border-slate-800 bg-slate-900 p-5 text-xs shadow-sm" aria-labelledby="resale-title">
    <h3 id="resale-title" class="flex items-center gap-2 text-sm font-bold text-white">
      <TrendingDown class="h-4 w-4 text-violet-400" aria-hidden="true" />
      {{ $t('dashboard.resale.title') }}
    </h3>

    <!-- Nothing to estimate yet -->
    <div v-if="missing" class="space-y-3 rounded-xl border border-dashed border-slate-700 bg-slate-950/40 p-4">
      <p class="font-semibold text-slate-200">{{ $t('dashboard.resale.missingTitle') }}</p>
      <p class="text-slate-400">{{ $t('dashboard.resale.missingBody') }}</p>
      <router-link to="/vehicles" class="btn btn-secondary">{{ $t('dashboard.resale.missingAction') }}</router-link>
    </div>

    <template v-else-if="residual">
      <div>
        <p class="text-slate-400">{{ $t('dashboard.resale.today') }}</p>
        <p class="text-3xl font-extrabold text-white">{{ formatAmount(residual.current_value, cur) }}</p>
      </div>

      <!-- From the purchase price to the expected resale value, with where the vehicle stands -->
      <div>
        <div class="h-2 overflow-hidden rounded-full bg-slate-800" role="meter" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="progressPct" :aria-label="$t('dashboard.resale.progress')">
          <div class="h-full rounded-full bg-violet-500 transition-all" :style="{ width: `${progressPct}%` }"></div>
        </div>
        <div class="mt-1.5 flex justify-between gap-2 text-slate-400">
          <span>{{ $t('dashboard.resale.purchase') }} <strong class="text-slate-200">{{ formatAmount(residual.purchase_price, cur) }}</strong></span>
          <span class="text-right">{{ $t('dashboard.resale.expectedResale', { months: residual.holding_months }) }} <strong class="text-slate-200">{{ formatAmount(residual.expected_resale_value, cur) }}</strong></span>
        </div>
        <p class="mt-2 text-slate-400">{{ $t('dashboard.resale.summary', { depreciation: formatAmount(residual.depreciation, cur), progress: progressPct }) }}</p>
      </div>

      <svg v-if="curvePath" viewBox="0 0 200 40" preserveAspectRatio="none" class="h-16 w-full" role="img" :aria-label="$t('dashboard.resale.curve')">
        <path :d="curvePath" fill="none" stroke="#a78bfa" stroke-width="1.5" vector-effect="non-scaling-stroke" />
        <line v-if="marker" :x1="marker.x" :x2="marker.x" y1="0" y2="40" stroke="#34d399" stroke-width="1.5" stroke-dasharray="3 3" vector-effect="non-scaling-stroke" />
      </svg>

      <details class="group rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2">
        <summary class="flex cursor-pointer list-none items-center justify-between gap-2 font-semibold text-slate-300">
          {{ $t('dashboard.resale.assumptions') }}
          <ChevronDown class="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden="true" />
        </summary>
        <div class="mt-3 space-y-3">
          <label class="block text-slate-400">
            {{ $t('dashboard.resale.expectedKm') }}
            <DistanceInput whole text v-model="expectedKm" min="0" step="1000" class="mt-1 block w-40 field" @change="load" />
            <span class="mt-1 block text-slate-500">{{ $t('dashboard.resale.expectedKmHint') }}</span>
          </label>
          <p class="text-slate-400">{{ $t('dashboard.resale.advancedHint') }}</p>
          <div class="flex flex-wrap items-end gap-3">
            <label class="text-slate-400">{{ $t('dashboard.resale.kmShare') }}
              <input v-model="kmShare" type="number" min="0" max="100" step="5" class="mt-1 block w-24 field" @change="load" />
            </label>
            <label v-if="hasBattery" for="residual-health-weight" class="text-slate-400">{{ $t('dashboard.resale.healthWeight') }}
              <NumberInput id="residual-health-weight" text v-model="healthWeight" min="0" max="2" class="mt-1 block w-24 field" @change="load" />
            </label>
          </div>
        </div>
      </details>
    </template>
  </section>
</template>
