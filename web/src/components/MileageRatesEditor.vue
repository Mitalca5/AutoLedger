<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { t } from '@/i18n'
import { api, type MileageRate } from '@/services/api'
import { Trash2 } from 'lucide-vue-next'

const emit = defineEmits<{ (e: 'change', labels: string[]): void }>()

const rates = ref<MileageRate[]>([])
const error = ref('')
const label = ref('')
const year = ref(new Date().getFullYear())
const fromKm = ref(0)
const toKm = ref<number | null>(null)
const rate = ref<number | null>(null)

async function load() {
  try {
    rates.value = (await api.getMileageRates()).rates
    emit('change', [...new Set(rates.value.map((r) => r.label))])
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function add() {
  error.value = ''
  try {
    await api.createMileageRate({
      label: label.value,
      year: year.value,
      from_km: fromKm.value || 0,
      to_km: toKm.value === null || (toKm.value as unknown) === '' ? null : toKm.value,
      rate_per_km: rate.value ?? 0,
    })
    fromKm.value = toKm.value ?? 0
    toKm.value = null
    rate.value = null
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function remove(id: string) {
  await api.deleteMileageRate(id)
  await load()
}

function describe(r: MileageRate) {
  return t('import.rateSlice', {
    from: r.from_km,
    to: r.to_km ?? t('import.rateOpenEnded'),
    rate: r.rate_per_km,
  })
}

onMounted(load)
</script>

<template>
  <div class="space-y-3 border-t border-slate-800 pt-3">
    <h3 class="text-sm font-semibold text-slate-200">{{ $t('import.exportRatesTitle') }}</h3>
    <p class="text-xs text-slate-400">{{ $t('import.exportRatesHint') }}</p>
    <ul v-if="rates.length" class="space-y-1 text-xs text-slate-300">
      <li v-for="r in rates" :key="r.id" class="flex items-center justify-between gap-2">
        <span>{{ r.label }} · {{ r.year }} · {{ describe(r) }}</span>
        <button type="button" class="text-slate-400 hover:text-danger-400" :aria-label="$t('import.rateDelete')" @click="remove(r.id)">
          <Trash2 class="w-4 h-4" />
        </button>
      </li>
    </ul>
    <div class="grid grid-cols-2 gap-2">
      <input v-model="label" :placeholder="$t('import.rateLabel')" :aria-label="$t('import.rateLabel')" maxlength="100" class="col-span-2 field" />
      <input v-model.number="year" type="number" :aria-label="$t('import.rateYear')" class="field" />
      <input v-model.number="rate" type="number" step="0.001" min="0" :placeholder="$t('import.ratePerKm')" :aria-label="$t('import.ratePerKm')" class="field" />
      <input v-model.number="fromKm" type="number" min="0" :placeholder="$t('import.rateFrom')" :aria-label="$t('import.rateFrom')" class="field" />
      <input v-model.number="toKm" type="number" min="1" :placeholder="$t('import.rateTo')" :aria-label="$t('import.rateTo')" class="field" />
    </div>
    <p v-if="error" class="text-xs text-danger-400" role="alert">{{ error }}</p>
    <button type="button" class="px-3 py-1.5 rounded-xl bg-slate-800 text-slate-200 text-xs font-semibold disabled:opacity-50" :disabled="!label || rate === null" :title="!label || rate === null ? $t('import.rateRequired') : undefined" @click="add">
      {{ $t('import.rateAdd') }}
    </button>
  </div>
</template>
