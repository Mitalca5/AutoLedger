import { canCharge, canRefuel } from '@/utils/vehicles'

export type TrackedSide = 'electric' | 'hybrid'

// A vehicle that records both fill-ups and charging sessions is compared as a hybrid.
export function trackedSide(powertrain: string | null | undefined): TrackedSide {
  return canCharge(powertrain) && canRefuel(powertrain) ? 'hybrid' : 'electric'
}

// The hybrid wording lives under `comparison.hybrid.*`, mirroring the electric keys.
export function sideKey(key: string, side: TrackedSide): string {
  return side === 'hybrid' ? key.replace(/^comparison\./, 'comparison.hybrid.') : key
}

// Projections compare two entered vehicles; a tracked comparison follows its reference vehicle.
export function scenarioSide(scenario: { mode?: string; vehicle_id?: string | null } | null | undefined, vehicles: { id: string; powertrain?: string }[]): TrackedSide {
  if (scenario?.mode !== 'RETROSPECTIVE') return 'electric'
  return trackedSide(vehicles.find((v) => v.id === scenario.vehicle_id)?.powertrain)
}
