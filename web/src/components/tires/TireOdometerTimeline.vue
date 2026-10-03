<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { displayDistanceToKm, formatDistance, kmToDisplayDistance } from '@/units'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { X } from 'lucide-vue-next'
import { buildTireTimeline, tickStep, type TimelineSegment, type TimelineTireInfo } from '../../utils/tireTimeline'
import { formatDate, getSeasonIcon } from '@/utils/tires'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

// The tires fitted over the odometer, one thin bar per axle or wheel. A segment shows its detail in a card anchored to
// it: on hover for a glance, pinned by a click (or a tap) to reach the tire sheets.
const props = defineProps<{ tires: any[]; currentOdometer: number }>()
const emit = defineEmits<{ (e: 'select-tire', tireId: string): void }>()

const timeline = computed(() => buildTireTimeline(props.tires, Math.round(props.currentOdometer || 0)))

const root = ref<HTMLElement | null>(null)
const hovered = ref<TimelineSegment | null>(null)
const pinned = ref<TimelineSegment | null>(null)
const shown = computed(() => pinned.value ?? hovered.value)

const pct = (km: number) => (timeline.value.maxKm > 0 ? (km / timeline.value.maxKm) * 100 : 0)
const fmtKm = (km: number) => formatDistance(km)

const hasData = computed(() => timeline.value.lanes.some((l) => l.segments.length > 0))
const hasGaps = computed(() => timeline.value.lanes.some((l) => l.gaps.length > 0))
const multiLane = computed(() => timeline.value.lanes.length > 1)
// The bar is thin; the clickable row around it is taller so it stays easy to hit
const barHeight = computed(() => (multiLane.value ? 'h-3' : 'h-4'))

const ticks = computed(() => {
  // Round steps in the account's unit, placed on the km scale of the bars
  const max = kmToDisplayDistance(timeline.value.maxKm)
  const step = tickStep(max)
  const out: number[] = []
  for (let d = 0; d <= max; d += step) out.push(displayDistanceToKm(d))
  return out
})

// The card opens under the middle of the segment, kept inside the chart near its ends
const cardPosition = computed(() => {
  const seg = shown.value
  if (!seg) return {}
  const mid = pct((seg.startKm + seg.endKm) / 2)
  return { '--card-left': `${mid}%`, '--card-shift': mid < 25 ? '0%' : mid > 75 ? '-100%' : '-50%' }
})

// Tires of a segment usually share their season and dates (a set fitted at once): say it once
const shared = computed(() => {
  const tires = shown.value?.tires ?? []
  const first = tires[0]
  if (!first) return null
  const same = tires.every((t) => t.mountedDate === first.mountedDate && t.dismountedDate === first.dismountedDate && t.season === first.season)
  return same ? first : null
})

function period(t: TimelineTireInfo) {
  return `${t.mountedDate ? formatDate(t.mountedDate) : '?'} → ${t.dismountedDate ? formatDate(t.dismountedDate) : ''}`
}

function onClick(seg: TimelineSegment) {
  pinned.value = pinned.value?.id === seg.id ? null : seg
}

// Lets go of the pinned segment; the hover state goes too, or the focus left on the segment would keep the card open
function unpin() {
  pinned.value = null
  hovered.value = null
}

function selectTire(tireId: string) {
  unpin()
  emit('select-tire', tireId)
}

useEscapeToClose(() => !!pinned.value, unpin)

// A click anywhere else than a segment or the card lets go of the pinned one
function onDocumentClick(e: MouseEvent) {
  if (pinned.value && !(e.target as HTMLElement | null)?.closest('[data-timeline-keep]')) unpin()
}
onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>

<template>
  <div ref="root" class="bg-slate-900 border border-slate-800 rounded-3xl p-5 space-y-3 shadow-sm">
    <div class="flex items-center justify-between flex-wrap gap-2">
      <h3 class="text-sm font-bold text-white">{{ $t('tires.tireOdometerTimeline.tiresFittedByMileage') }}</h3>
      <span class="text-xs text-slate-400">0 → {{ fmtKm(timeline.maxKm) }}</span>
    </div>

    <div v-if="!hasData" class="p-6 text-center text-xs text-slate-400 bg-slate-950/40 rounded-2xl">
      {{ $t('tires.tireOdometerTimeline.noFittingSessionWithMileage') }}
    </div>

    <template v-else>
      <div class="space-y-0.5">
        <div v-for="lane in timeline.lanes" :key="lane.id" class="flex items-center gap-3">
          <span
            v-if="multiLane"
            class="w-9 sm:w-32 shrink-0 text-xs text-slate-400 truncate"
            :title="lane.label"
          >
            <span class="sm:hidden">{{ lane.shortLabel }}</span>
            <span class="hidden sm:inline">{{ lane.label }}</span>
          </span>
          <div class="relative flex-1 h-5">
            <div class="absolute inset-x-0 top-1/2 -translate-y-1/2 rounded-full overflow-hidden bg-slate-950 border border-slate-800" :class="barHeight">
              <div
                v-for="(gap, i) in lane.gaps"
                :key="`gap-${i}`"
                class="absolute top-0 h-full"
                :style="{
                  left: pct(gap.startKm) + '%',
                  width: pct(gap.endKm - gap.startKm) + '%',
                  backgroundImage: 'repeating-linear-gradient(45deg, #334155 0 3px, transparent 3px 6px)',
                }"
                :title="$t('tires.tireOdometerTimeline.noTire', { from: fmtKm(gap.startKm), to: fmtKm(gap.endKm) })"
              ></div>
              <div
                v-for="seg in lane.segments"
                :key="seg.id"
                class="absolute top-0 h-full border-r border-slate-950 transition-[filter]"
                :class="[
                  { 'ring-2 ring-white ring-inset': pinned?.id === seg.id },
                  shown?.id === seg.id ? 'brightness-125' : '',
                ]"
                :style="{ left: pct(seg.startKm) + '%', width: pct(seg.endKm - seg.startKm) + '%', backgroundColor: seg.color }"
              ></div>
            </div>
            <button
              v-for="seg in lane.segments"
              :key="`hit-${seg.id}`"
              type="button"
              data-timeline-keep
              class="absolute top-0 h-full rounded-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
              :style="{ left: pct(seg.startKm) + '%', width: pct(seg.endKm - seg.startKm) + '%' }"
              :aria-label="$t('tires.tireOdometerTimeline.segment', { lane: lane.label, tire: seg.tires[0].label, from: fmtKm(seg.startKm), to: fmtKm(seg.endKm) })"
              :aria-pressed="pinned?.id === seg.id"
              @mouseenter="hovered = seg"
              @mouseleave="hovered = null"
              @focus="hovered = seg"
              @blur="hovered = null"
              @click="onClick(seg)"
            ></button>
          </div>
        </div>
      </div>

      <!-- Axis; the detail card hangs from it, under the segment it describes -->
      <div class="relative h-4 text-xs text-slate-400" :class="multiLane ? 'ml-12 sm:ml-[8.75rem]' : ''">
        <span v-for="km in ticks" :key="km" class="absolute -translate-x-1/2 first:translate-x-0" :style="{ left: pct(km) + '%' }">
          {{ (km / 1000).toLocaleString(intlLocale()) }}k
        </span>

        <div
          v-if="shown"
          data-timeline-keep
          class="absolute top-full mt-1 z-30 inset-x-0 sm:inset-x-auto sm:w-80 sm:left-[var(--card-left)] sm:[transform:translateX(var(--card-shift))] rounded-2xl border border-slate-700 bg-slate-950 shadow-xl shadow-black/40 p-3 text-xs space-y-2"
          :class="pinned ? '' : 'pointer-events-none'"
          :style="cardPosition"
          role="dialog"
        >
          <div class="flex items-start justify-between gap-2">
            <div>
              <div class="text-slate-200 font-semibold">
                {{ fmtKm(shown.startKm) }} → {{ fmtKm(shown.endKm) }}
                <span class="text-slate-400 font-normal">({{ fmtKm(shown.endKm - shown.startKm) }})</span>
              </div>
              <div v-if="shared" class="mt-0.5 flex items-center gap-1.5 text-xs text-slate-400">
                <component :is="getSeasonIcon(shared.season).icon" class="w-3 h-3" :class="getSeasonIcon(shared.season).color" />
                {{ getSeasonIcon(shared.season).label }} · {{ period(shared) }}
                <span v-if="shared.ongoing" class="text-emerald-400">{{ $t('tires.tireOdometerTimeline.ongoing') }}</span>
              </div>
            </div>
            <button v-if="pinned" type="button" class="p-0.5 text-slate-400 hover:text-white shrink-0" :title="$t('common.close')" @click="unpin">
              <X class="w-3.5 h-3.5" />
            </button>
          </div>
          <ul class="space-y-1.5">
            <li v-for="t in shown.tires" :key="t.tireId + t.position" class="text-slate-300">
              <div class="flex items-center justify-between gap-2">
                <span class="min-w-0">
                  <span class="font-mono text-rose-400 mr-1">{{ t.position }}</span>{{ t.label }}
                  <span class="text-slate-400 font-mono">{{ t.dimension }}</span>
                </span>
                <button v-if="pinned" type="button" class="text-rose-400 hover:text-rose-300 font-semibold shrink-0" @click="selectTire(t.tireId)">
                  {{ $t('tires.tireOdometerTimeline.details') }}
                </button>
              </div>
              <div v-if="!shared" class="flex items-center gap-1.5 text-xs text-slate-400">
                <component :is="getSeasonIcon(t.season).icon" class="w-3 h-3" :class="getSeasonIcon(t.season).color" />
                {{ period(t) }}
                <span v-if="t.ongoing" class="text-emerald-400">{{ $t('tires.tireOdometerTimeline.ongoing') }}</span>
              </div>
            </li>
          </ul>
        </div>
      </div>

      <div class="flex flex-wrap gap-x-4 gap-y-1 pt-1">
        <span v-for="l in timeline.legend" :key="l.key" class="flex items-center gap-1.5 text-xs text-slate-300">
          <span class="w-2.5 h-2.5 rounded-sm" :style="{ backgroundColor: l.color }"></span>{{ l.key }}
        </span>
        <span v-if="hasGaps" class="flex items-center gap-1.5 text-xs text-slate-400">
          <span class="w-2.5 h-2.5 rounded-sm bg-slate-600"></span>{{ $t('tires.tireOdometerTimeline.noTireRecorded') }}
        </span>
      </div>
    </template>
  </div>
</template>
