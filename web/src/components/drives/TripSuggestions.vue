<script setup lang="ts">
import { computed, ref } from 'vue'
import { formatDistance } from '@/units'
import { intlLocale } from '@/i18n'
import { Sparkles, Zap, Timer } from 'lucide-vue-next'
import { formatTripDates } from '@/utils/drives'
import { useVehicleStore } from '@/stores/vehicle'
import QualifyActions from '@/components/drives/QualifyActions.vue'

// The "to qualify" queue of trips: chains of drives that look like one trip (short stops, or a charge in between).
// Each one is either turned into a trip or ruled out ("not a trip"), like a drive is given a toll or marked without one;
// opening one shows its costs before deciding.
const props = defineProps<{ suggestions: any[]; busyKey: string | null }>()
const emit = defineEmits<{ create: [suggestion: any]; dismiss: [suggestion: any]; open: [suggestion: any] }>()
const vehicleStore = useVehicleStore()

const PAGE = 20
const shown = ref(PAGE)
const visible = computed(() => props.suggestions.slice(0, shown.value))

const key = (s: any) => s.drive_ids[0]
const route = (s: any) => [s.start_address, s.end_address].filter(Boolean).join(' → ')
</script>

<template>
  <div v-if="suggestions.length" class="space-y-2">
    <div class="flex items-start gap-2 text-xs text-slate-400 px-1">
      <Sparkles class="w-4 h-4 text-warning-400 shrink-0 mt-0.5" />
      <div>
        <span class="font-bold text-white">{{ $t('drives.tripSuggestions.title', { count: suggestions.length }) }}</span>
        <span class="block">{{ $t('drives.tripSuggestions.help', { min: formatDistance(50) }) }}</span>
      </div>
    </div>

    <div
      v-for="s in visible"
      :key="key(s)"
      @click="emit('open', s)"
      class="bg-slate-900/60 border border-dashed border-warning-500/30 hover:border-warning-500/60 p-4 rounded-2xl flex flex-col lg:flex-row lg:items-center justify-between gap-3 cursor-pointer transition-colors"
    >
      <div class="min-w-0">
        <div class="flex items-center gap-2 flex-wrap mb-1">
          <span class="text-xs font-semibold text-slate-400">{{ formatTripDates({ start_time: s.start_time, end_time: s.end_time }) }}</span>
          <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60">
            {{ formatDistance(s.distance_km) }}
          </span>
          <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/30">
            {{ $t('drives.tripGroupsPanel.legS', { length: s.drive_ids.length }) }}
          </span>
          <span
            class="text-xs px-2.5 py-0.5 rounded-full font-semibold border flex items-center gap-1"
            :class="s.reason === 'CHARGE' ? 'bg-info-500/10 text-info-300 border-info-500/30' : 'bg-warning-500/10 text-warning-300 border-warning-500/30'"
          >
            <component :is="s.reason === 'CHARGE' ? Zap : Timer" class="w-3 h-3" />
            {{ s.reason === 'CHARGE' ? $t('drives.tripSuggestions.reasonCharge') : $t('drives.tripSuggestions.reasonPause') }}
          </span>
        </div>
        <div class="text-sm font-bold text-white truncate">{{ route(s) || $t('drives.tripSuggestions.unnamed') }}</div>
      </div>
      <QualifyActions
        v-if="vehicleStore.canEdit"
        :busy="busyKey === key(s)"
        :primary-label="$t('drives.tripSuggestions.create')"
        :primary-title="$t('drives.tripSuggestions.createTitle')"
        :secondary-label="$t('drives.tripSuggestions.dismiss')"
        :secondary-title="$t('drives.tripSuggestions.dismissTitle')"
        @click.stop
        @primary="emit('create', s)"
        @secondary="emit('dismiss', s)"
      />
    </div>

    <button
      v-if="suggestions.length > visible.length"
      type="button"
      @click="shown += PAGE"
      class="w-full py-2 text-xs font-semibold text-slate-400 hover:text-white bg-slate-900/60 border border-slate-800 rounded-xl transition-colors"
    >
      {{ $t('drives.tripSuggestions.showMore', { count: Math.min(PAGE, suggestions.length - visible.length) }) }}
    </button>
  </div>
</template>
