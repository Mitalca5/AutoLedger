<script setup lang="ts">
import { computed } from 'vue'
import { Gauge, Calculator } from 'lucide-vue-next'
import { currencySymbol } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'

defineEmits<{ start: [mode: 'RETROSPECTIVE' | 'PROJECTION'] }>()

const vehicleStore = useVehicleStore()
const canTrack = computed(() => !!vehicleStore.activeVehicle && vehicleStore.canCharge)
const cur = computed(() => currencySymbol(vehicleStore.currency))

// Illustrative figures only: nothing here is read from, or saved to, the user's data.
const example = [
  { key: 'energy', electric: 4100, ice: 8300 },
  { key: 'maintenance', electric: 1600, ice: 2250 },
  { key: 'insurance', electric: 3000, ice: 3100 },
  { key: 'depreciation', electric: 9200, ice: 11500 },
] as const
const total = (side: 'electric' | 'ice') => example.reduce((sum, row) => sum + row[side], 0)
const money = (n: number) => `${n.toLocaleString()} ${cur.value}`
</script>

<template>
  <div class="space-y-4">
    <p class="text-sm text-slate-400">{{ $t('comparison.emptyState.intro') }}</p>

    <div class="grid gap-3 md:grid-cols-2">
      <section class="rounded-2xl border border-slate-800 bg-slate-900 p-4 space-y-2" aria-labelledby="cmp-empty-tracked">
        <h3 id="cmp-empty-tracked" class="flex items-center gap-2 text-sm font-bold text-white"><Gauge class="w-4 h-4 text-cyan-400" /> {{ $t('comparison.emptyState.trackedTitle') }}</h3>
        <p class="text-sm text-slate-400">{{ $t('comparison.emptyState.trackedText') }}</p>
        <p v-if="!canTrack" class="text-xs text-warning-400">{{ $t('comparison.emptyState.trackedUnavailable') }} <router-link to="/vehicles" class="underline">{{ $t('comparison.emptyState.vehiclesLink') }}</router-link></p>
        <button type="button" class="btn btn-primary" :disabled="!canTrack" @click="$emit('start', 'RETROSPECTIVE')">{{ $t('comparison.emptyState.trackedAction') }}</button>
      </section>
      <section class="rounded-2xl border border-slate-800 bg-slate-900 p-4 space-y-2" aria-labelledby="cmp-empty-projection">
        <h3 id="cmp-empty-projection" class="flex items-center gap-2 text-sm font-bold text-white"><Calculator class="w-4 h-4 text-cyan-400" /> {{ $t('comparison.emptyState.projectionTitle') }}</h3>
        <p class="text-sm text-slate-400">{{ $t('comparison.emptyState.projectionText') }}</p>
        <button type="button" class="btn btn-secondary" @click="$emit('start', 'PROJECTION')">{{ $t('comparison.emptyState.projectionAction') }}</button>
      </section>
    </div>

    <p v-if="canTrack" class="text-xs text-slate-400">
      {{ $t('comparison.emptyState.dataHint') }}
      <router-link to="/energy" class="underline">{{ $t('comparison.emptyState.energyLink') }}</router-link> ·
      <router-link to="/odometer" class="underline">{{ $t('comparison.emptyState.odometerLink') }}</router-link>
    </p>

    <section class="rounded-2xl border border-dashed border-slate-700 bg-slate-900/50 p-4" aria-labelledby="cmp-empty-example">
      <div class="flex items-center justify-between gap-2">
        <h3 id="cmp-empty-example" class="text-sm font-bold text-white">{{ $t('comparison.emptyState.exampleTitle') }}</h3>
        <span class="rounded-full border border-slate-700 px-2 py-0.5 text-xs text-slate-400">{{ $t('comparison.emptyState.exampleBadge') }}</span>
      </div>
      <p class="mt-1 text-xs text-slate-400">{{ $t('comparison.emptyState.exampleText') }}</p>
      <table class="mt-3 w-full text-sm">
        <thead>
          <tr class="text-left text-xs text-slate-400">
            <th scope="col" class="py-1 font-medium">{{ $t('comparison.comparisonView.category') }}</th>
            <th scope="col" class="py-1 text-right font-medium">{{ $t('comparison.comparisonView.electric') }}</th>
            <th scope="col" class="py-1 text-right font-medium">{{ $t('comparison.comparisonView.combustion') }}</th>
          </tr>
        </thead>
        <tbody class="text-slate-300">
          <tr v-for="row in example" :key="row.key" class="border-t border-slate-800">
            <th scope="row" class="py-1 text-left font-normal">{{ $t(`comparison.emptyState.rows.${row.key}`) }}</th>
            <td class="py-1 text-right tabular-nums">{{ money(row.electric) }}</td>
            <td class="py-1 text-right tabular-nums">{{ money(row.ice) }}</td>
          </tr>
          <tr class="border-t border-slate-700 font-semibold text-white">
            <th scope="row" class="py-1 text-left">{{ $t('comparison.comparisonView.total') }}</th>
            <td class="py-1 text-right tabular-nums">{{ money(total('electric')) }}</td>
            <td class="py-1 text-right tabular-nums">{{ money(total('ice')) }}</td>
          </tr>
        </tbody>
      </table>
      <p class="mt-2 text-xs text-slate-400">{{ $t('comparison.emptyState.breakEven') }}</p>
    </section>
  </div>
</template>
