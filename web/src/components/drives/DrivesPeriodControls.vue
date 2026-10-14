<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { computed } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import {
  currentYearMonth,
  presetRange,
  shiftAnchor,
  todayIso,
  unitRange,
  type PeriodMode,
  type PeriodPreset,
  type PeriodUnit,
} from '@/utils/drives'

// The period of the drives list: a unit (all, day, week, month, year, range) stepped with the arrows, as in Home
// Assistant's energy dashboard, with a button back to the current one; a range has ready-made periods.
// Every change is reported with change so the page reloads from the first page.
const emit = defineEmits<{ change: [] }>()
const periodMode = defineModel<PeriodMode>('periodMode', { required: true })
const selectedMonth = defineModel<string>('selectedMonth', { required: true })
const anchorDay = defineModel<string>('anchorDay', { required: true })
const customFrom = defineModel<string>('customFrom', { required: true })
const customTo = defineModel<string>('customTo', { required: true })

const MODES: PeriodMode[] = ['ALL', 'DAY', 'WEEK', 'MONTH', 'YEAR', 'CUSTOM']

// DAY, WEEK, MONTH and YEAR step with the arrows; ALL and CUSTOM do not
const unit = computed<PeriodUnit | null>(() => (periodMode.value === 'ALL' || periodMode.value === 'CUSTOM' ? null : periodMode.value))
// The reference day of the current unit: the month picker keeps its own value for MONTH
const reference = computed(() => (periodMode.value === 'MONTH' ? `${selectedMonth.value}-01` : anchorDay.value))
const isCurrent = computed(() => {
  if (!unit.value) return true
  const now = unitRange(unit.value, todayIso())
  const cur = unitRange(unit.value, reference.value)
  return now.from === cur.from && now.to === cur.to
})

const dayFormat = (iso: string, withYear: boolean) => {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d).toLocaleDateString(intlLocale(), { day: 'numeric', month: 'short', ...(withYear ? { year: 'numeric' } : {}) })
}
const unitLabel = computed(() => {
  if (!unit.value) return ''
  const r = unitRange(unit.value, reference.value)
  if (unit.value === 'WEEK') return `${dayFormat(r.from, false)} – ${dayFormat(r.to, true)}`
  if (unit.value === 'YEAR') return r.from.slice(0, 4)
  return ''
})

function step(dir: -1 | 1) {
  if (!unit.value) return
  const next = shiftAnchor(unit.value, reference.value, dir)
  anchorDay.value = next
  selectedMonth.value = next.slice(0, 7)
  emit('change')
}

function resetToCurrent() {
  anchorDay.value = todayIso()
  selectedMonth.value = currentYearMonth()
  emit('change')
}

function setPeriodMode(mode: PeriodMode) {
  // Keep the day the user was looking at when moving between units
  if (periodMode.value === 'MONTH' && mode !== 'MONTH') anchorDay.value = selectedMonth.value === currentYearMonth() ? todayIso() : `${selectedMonth.value}-01`
  if (mode === 'MONTH') selectedMonth.value = anchorDay.value.slice(0, 7)
  periodMode.value = mode
  emit('change')
}

function onAnchorChange() {
  selectedMonth.value = anchorDay.value.slice(0, 7)
  emit('change')
}

function onMonthChange() {
  anchorDay.value = `${selectedMonth.value}-01`
  emit('change')
}

const PRESETS: PeriodPreset[] = ['lastMonth', 'last30', 'last90', 'thisYear']
function applyPreset(preset: PeriodPreset) {
  const { from, to } = presetRange(preset)
  customFrom.value = from
  customTo.value = to
  emit('change')
}

function onCustomDateChange() {
  emit('change')
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <div class="flex items-center rounded-xl border border-slate-800/80 bg-slate-950 p-1" role="group" :aria-label="$t('drives.drivesToolbar.period')">
      <button
        v-for="m in MODES"
        :key="m"
        type="button"
        @click="setPeriodMode(m)"
        :aria-pressed="periodMode === m"
        class="rounded-lg border px-2.5 py-1 text-xs font-semibold transition-colors"
        :class="periodMode === m ? 'border-slate-600 bg-slate-700 text-white' : 'border-transparent text-slate-400 hover:text-white'"
      >
        {{ $t(`drives.drivesToolbar.units.${m}`) }}
      </button>
    </div>

    <!-- Stepping through the units -->
    <div v-if="unit" class="flex items-center gap-1 rounded-xl border border-slate-800/80 bg-slate-950 px-1.5 py-1">
      <button
        type="button"
        @click="step(-1)"
        class="tap rounded-lg p-1 text-slate-400 transition-colors hover:bg-slate-800 hover:text-white"
        :title="$t('drives.drivesToolbar.previous')" :aria-label="$t('drives.drivesToolbar.previous')"
      >
        <ChevronLeft class="h-4 w-4" />
      </button>
      <div v-if="unit === 'MONTH'" class="w-40 sm:w-44">
        <label for="drives-month-select" class="sr-only">{{ $t('drives.drivesToolbar.selectTheMonth') }}</label>
        <AppDatePicker id="drives-month-select" v-model="selectedMonth" month-picker size="xs" :clearable="false" @change="onMonthChange" />
      </div>
      <div v-else-if="unit === 'DAY'" class="w-36">
        <label for="drives-day-select" class="sr-only">{{ $t('drives.drivesToolbar.units.DAY') }}</label>
        <AppDatePicker id="drives-day-select" v-model="anchorDay" size="xs" :clearable="false" @change="onAnchorChange" />
      </div>
      <span v-else class="min-w-[8rem] px-2 text-center text-xs font-semibold text-slate-200" aria-live="polite">{{ unitLabel }}</span>
      <button
        type="button"
        @click="step(1)"
        class="tap rounded-lg p-1 text-slate-400 transition-colors hover:bg-slate-800 hover:text-white"
        :title="$t('drives.drivesToolbar.next')" :aria-label="$t('drives.drivesToolbar.next')"
      >
        <ChevronRight class="h-4 w-4" />
      </button>
    </div>
    <button
      v-if="unit && !isCurrent"
      type="button"
      @click="resetToCurrent"
      class="btn btn-secondary"
    >
      {{ $t(`drives.drivesToolbar.current.${unit}`) }}
    </button>

    <!-- Custom range, with ready-made ranges -->
    <div v-if="periodMode === 'CUSTOM'" class="flex flex-wrap items-center gap-2 rounded-xl border border-slate-800/80 bg-slate-950 px-2.5 py-1 text-xs">
      <label for="drives-filter-from" class="shrink-0 text-slate-400">{{ $t('drives.drivesToolbar.from') }}</label>
      <div class="w-36">
        <AppDatePicker id="drives-filter-from" v-model="customFrom" size="xs" @change="onCustomDateChange" />
      </div>
      <label for="drives-filter-to" class="shrink-0 text-slate-400">{{ $t('drives.drivesToolbar.to') }}</label>
      <div class="w-36">
        <AppDatePicker id="drives-filter-to" v-model="customTo" size="xs" @change="onCustomDateChange" />
      </div>
      <div class="flex items-center gap-1 border-l border-slate-800 pl-2">
        <button
          v-for="p in PRESETS"
          :key="p"
          type="button"
          class="rounded-lg border border-transparent px-2 py-1 font-semibold text-slate-400 transition-colors hover:border-slate-700 hover:text-white"
          @click="applyPreset(p)"
        >
          {{ $t(`drives.drivesToolbar.presets.${p}`) }}
        </button>
      </div>
    </div>
  </div>
</template>
