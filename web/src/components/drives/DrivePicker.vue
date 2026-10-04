<script setup lang="ts">
import { ref, watch } from 'vue'
import { formatDistance } from '@/units'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { DRIVE_WINDOW_DAYS, driveWindow, formatDriveTime, pickerDrives, shiftDay, toDateInputString } from '@/utils/carpool'

// The drives to tick to build a carpool or a trip. It lists the drives around `anchorDate` (the date of the record
// being edited), or the latest ones without a date, with a date field and arrows to move the window. The selected
// drives are always listed, even outside the window. The parent owns the selection: it gets `toggle`.
const props = defineProps<{ vehicleId: string; selectedIds: string[]; anchorDate: string }>()
const emit = defineEmits<{ toggle: [driveId: string]; loaded: [drives: any[]] }>()

const pickerDate = ref('')
const windowDrives = ref<any[]>([])
const drives = ref<any[]>([])
const known = new Map<string, any>()
// Drives kept in the list once selected, until the window moves, so unticking one does not make it vanish
const sticky = new Set<string>()

async function fetchMissingSelected() {
  const missing = props.selectedIds.filter((id) => !known.has(id))
  await Promise.all(
    missing.map(async (id) => {
      const one = await api.getDrives(props.vehicleId, { driveId: id, limit: 1 })
      if (one.drives?.[0]) known.set(id, one.drives[0])
    }),
  )
}

function merge() {
  for (const id of props.selectedIds) sticky.add(id)
  drives.value = pickerDrives(windowDrives.value, known, [...sticky])
  emit('loaded', drives.value)
}

async function load() {
  if (!props.vehicleId) return
  try {
    const params = pickerDate.value ? { ...driveWindow(pickerDate.value), limit: 200 } : { limit: 200 }
    const res = await api.getDrives(props.vehicleId, params)
    windowDrives.value = res.drives || []
    for (const d of windowDrives.value) known.set(d.id, d)
    sticky.clear()
    await fetchMissingSelected()
    merge()
  } catch (err) {
    console.error('Failed to load the drives', err)
  }
}

function shiftWindow(days: number) {
  pickerDate.value = shiftDay(pickerDate.value || toDateInputString(new Date()), days)
  load()
}

watch(
  () => props.anchorDate,
  (date) => {
    pickerDate.value = date
    load()
  },
  { immediate: true },
)

// The selection also changes from outside (an estimate, a trip group): its drives are listed too
watch(
  () => props.selectedIds.join(','),
  async () => {
    try {
      await fetchMissingSelected()
      merge()
    } catch (err) {
      console.error('Failed to load the selected drives', err)
    }
  },
)
</script>

<template>
  <div class="space-y-1.5">
    <!-- Which drives are listed: the latest ones, or those around a date -->
    <div class="flex items-center gap-1.5 text-xs text-slate-400">
      <button type="button" @click="shiftWindow(-2 * DRIVE_WINDOW_DAYS)" class="tap p-1 rounded-lg hover:bg-slate-800 hover:text-white" :title="$t('drives.drivePicker.earlierDrives')">
        <ChevronLeft class="w-4 h-4" />
      </button>
      <div class="w-44">
        <AppDatePicker
          id="drive-picker-window"
          v-model="pickerDate"
          size="xs"
          :clearable="true"
          :placeholder="$t('drives.drivePicker.latestDrives')"
          @change="load"
        />
      </div>
      <button type="button" @click="shiftWindow(2 * DRIVE_WINDOW_DAYS)" class="tap p-1 rounded-lg hover:bg-slate-800 hover:text-white" :title="$t('drives.drivePicker.laterDrives')">
        <ChevronRight class="w-4 h-4" />
      </button>
      <span v-if="pickerDate" class="text-xs text-slate-400">{{ $t('drives.drivePicker.aroundTheDate', { days: DRIVE_WINDOW_DAYS }) }}</span>
    </div>
    <p v-if="!drives.length" class="text-xs text-slate-400">{{ $t('drives.drivePicker.noDriveInThisPeriod') }}</p>
    <div class="max-h-44 overflow-y-auto space-y-1 pr-1">
      <label
        v-for="d in drives"
        :key="d.id"
        class="w-full flex items-center justify-between gap-3 p-2 rounded-lg text-xs border text-left cursor-pointer transition-colors"
        :class="selectedIds.includes(d.id) ? 'bg-rose-500/10 border-rose-500/40 text-rose-100' : 'bg-slate-800/60 border-slate-700 text-slate-300 hover:bg-slate-800'"
      >
        <span class="flex items-center gap-2 truncate">
          <input type="checkbox" class="select-box" :checked="selectedIds.includes(d.id)" @change="emit('toggle', d.id)" />
          <span class="truncate">{{ formatDriveTime(d.start_time) }}{{ $t('drives.tripGroupsPanel.dateSeparator') }}{{ (d.start_address || $t('drives.driveCostModal.start')).split(',')[0] }} → {{ (d.end_address || $t('drives.driveCostModal.end')).split(',')[0] }}</span>
        </span>
        <span class="font-mono text-xs text-slate-400 shrink-0">{{ formatDistance(Number(d.distance_km)) }}</span>
      </label>
    </div>
  </div>
</template>
