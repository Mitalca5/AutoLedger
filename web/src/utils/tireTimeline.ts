import { t } from '@/i18n'
export interface TimelineTireInfo {
  tireId: string
  label: string
  dimension: string
  position: string
  startKm: number
  endKm: number
  ongoing: boolean
  season: string
  mountedDate: string | null
  dismountedDate: string | null
}

export interface TimelineSegment {
  id: string
  startKm: number
  endKm: number
  colorKey: string
  color: string
  tires: TimelineTireInfo[]
}

export interface TimelineGap {
  startKm: number
  endKm: number
}

export interface TimelineLane {
  id: string
  label: string
  // Fits a narrow label column (an axle or wheel code)
  shortLabel: string
  segments: TimelineSegment[]
  gaps: TimelineGap[]
}

export interface TireTimeline {
  maxKm: number
  lanes: TimelineLane[]
  legend: { key: string; color: string }[]
}

// Sessions whose bounds differ by less than this are the same physical mounting (a set of tires).
const GROUP_TOLERANCE_KM = 50

const PALETTE = ['#f43f5e', '#38bdf8', '#34d399', '#fbbf24', '#a78bfa', '#fb923c', '#2dd4bf', '#f472b6', '#a3e635', '#60a5fa']

const WHEELS = ['FL', 'FR', 'RL', 'RR'] as const
const wheelLabel = (wheel: string): string => t(`tires.timeline.wheels.${wheel}`)

interface FlatSession extends TimelineTireInfo {
  colorKey: string
}

function flattenSessions(tires: any[], currentKm: number): FlatSession[] {
  const flat: FlatSession[] = []
  for (const t of tires) {
    for (const s of t.sessions || []) {
      const start = Number(s.mounted_odometer)
      if (!Number.isFinite(start)) continue
      const ongoing = !s.dismounted_date
      let end = ongoing ? currentKm : Number(s.dismounted_odometer)
      if (!ongoing && !(end > start) && s.distance_km > 0) end = start + Number(s.distance_km)
      if (!(end > start)) continue
      const label = `${t.tire.brand} ${t.tire.model}`.trim()
      flat.push({
        tireId: t.tire.id,
        label,
        dimension: t.tire.dimension,
        position: s.position,
        startKm: start,
        endKm: end,
        ongoing,
        season: t.tire.season,
        mountedDate: s.mounted_date ?? null,
        dismountedDate: s.dismounted_date ?? null,
        colorKey: `${label} ${t.tire.dimension}`,
      })
    }
  }
  return flat.sort((a, b) => a.startKm - b.startKm || a.endKm - b.endKm)
}

// Groups sessions covering the same range into one segment and clips overlaps so a lane stays a single line.
function buildLane(
  id: string,
  label: string,
  shortLabel: string,
  sessions: FlatSession[],
  colorFor: (key: string) => string,
  maxKm: number,
): TimelineLane {
  const grouped: TimelineSegment[] = []
  for (const s of sessions) {
    const last = grouped[grouped.length - 1]
    if (
      last &&
      Math.abs(last.startKm - s.startKm) <= GROUP_TOLERANCE_KM &&
      Math.abs(last.endKm - s.endKm) <= GROUP_TOLERANCE_KM
    ) {
      last.tires.push(s)
      last.endKm = Math.max(last.endKm, s.endKm)
      continue
    }
    grouped.push({
      id: `${id}-${s.tireId}-${s.startKm}`,
      startKm: s.startKm,
      endKm: s.endKm,
      colorKey: s.colorKey,
      color: colorFor(s.colorKey),
      tires: [s],
    })
  }

  const segments: TimelineSegment[] = []
  const gaps: TimelineGap[] = []
  let cursor = 0
  for (const seg of grouped) {
    if (seg.startKm > cursor) gaps.push({ startKm: cursor, endKm: seg.startKm })
    const start = Math.max(seg.startKm, cursor)
    if (seg.endKm > start) {
      segments.push({ ...seg, startKm: start })
      cursor = seg.endKm
    }
  }
  if (cursor < maxKm) gaps.push({ startKm: cursor, endKm: maxKm })
  return { id, label, shortLabel, segments, gaps }
}

function sameCoverage(a: TimelineLane, b: TimelineLane): boolean {
  return (
    a.segments.length === b.segments.length &&
    a.segments.every(
      (s, i) =>
        s.colorKey === b.segments[i].colorKey &&
        Math.abs(s.startKm - b.segments[i].startKm) <= GROUP_TOLERANCE_KM &&
        Math.abs(s.endKm - b.segments[i].endKm) <= GROUP_TOLERANCE_KM,
    )
  )
}

function mergeLanes(id: string, label: string, shortLabel: string, a: TimelineLane, b: TimelineLane): TimelineLane {
  return {
    id,
    label,
    shortLabel,
    gaps: a.gaps,
    segments: a.segments.map((s, i) => {
      const tires = [...s.tires, ...b.segments[i].tires]
      const unique = tires.filter((t, j) => tires.findIndex((o) => o.tireId === t.tireId && o.position === t.position) === j)
      return { ...s, id: `${id}-${i}`, tires: unique }
    }),
  }
}

// Splits only as far as the data requires: one lane if every wheel had the same tires over the same
// km, one per axle if left/right always match, otherwise one per wheel (per axle where sides match).
export function buildTireTimeline(tires: any[], currentOdometer: number): TireTimeline {
  const flat = flattenSessions(tires, currentOdometer)
  const maxKm = Math.max(currentOdometer, ...flat.map((s) => s.endKm), 0)

  const colorByKey = new Map<string, string>()
  const colorFor = (key: string) => {
    if (!colorByKey.has(key)) colorByKey.set(key, PALETTE[colorByKey.size % PALETTE.length])
    return colorByKey.get(key)!
  }

  // Sessions without a precise wheel position apply to every wheel.
  const wheel = Object.fromEntries(
    WHEELS.map((w) => [
      w,
      buildLane(
        w,
        `${wheelLabel(w)} (${w})`,
        w,
        flat.filter((s) => s.position === w || !(WHEELS as readonly string[]).includes(s.position)),
        colorFor,
        maxKm,
      ),
    ]),
  ) as Record<(typeof WHEELS)[number], TimelineLane>

  const front = sameCoverage(wheel.FL, wheel.FR) ? [mergeLanes('front', t('tires.timeline.front'), t('tires.timeline.frontShort'), wheel.FL, wheel.FR)] : [wheel.FL, wheel.FR]
  const rear = sameCoverage(wheel.RL, wheel.RR) ? [mergeLanes('rear', t('tires.timeline.rear'), t('tires.timeline.rearShort'), wheel.RL, wheel.RR)] : [wheel.RL, wheel.RR]

  let lanes = [...front, ...rear]
  if (front.length === 1 && rear.length === 1 && sameCoverage(front[0], rear[0])) {
    lanes = [mergeLanes('all', t('tires.timeline.all'), t('tires.timeline.allShort'), front[0], rear[0])]
  }

  const legend = [...colorByKey.entries()].map(([key, color]) => ({ key, color }))
  return { maxKm, lanes, legend }
}

export function tickStep(maxKm: number): number {
  if (maxKm <= 20000) return 5000
  if (maxKm <= 60000) return 10000
  return 20000
}

export interface TimelineTireGroup extends TimelineTireInfo {
  count: number
}

/** Merges the tires of a tooltip that read the same (a set of four fitted without wheel positions) into one line with a count. */
export function groupTimelineTires(tires: TimelineTireInfo[]): TimelineTireGroup[] {
  const groups = new Map<string, TimelineTireGroup>()
  for (const tire of tires) {
    const wheel = (WHEELS as readonly string[]).includes(tire.position)
    const key = wheel ? `${tire.tireId}|${tire.position}` : [tire.position, tire.label, tire.dimension, tire.season, tire.mountedDate, tire.dismountedDate, tire.ongoing].join('|')
    const group = groups.get(key)
    if (group) group.count++
    else groups.set(key, { ...tire, count: 1 })
  }
  return [...groups.values()]
}
