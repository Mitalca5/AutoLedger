import { t } from '@/i18n'
// Pure helpers of the quick entry sheet: payload builders, tariff memory and small numeric utilities.
// Kept free of Vue and of the API layer so they can be unit tested.

export type QuickKind = 'PENDING' | 'CHARGE' | 'FUEL' | 'EXPENSE'

export type ExpenseType = 'TOLL' | 'PARKING' | 'FERRY' | 'OTHER'

// Last values entered on a vehicle, used to prefill the next entry.
export interface QuickMemory {
  pricePerKwh?: number
  address?: string
}

export interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

const MEMORY_PREFIX = 'autoledger.quickadd.'
const LEGACY_MEMORY_PREFIX = 'teslacost.quickadd.'

// Parses a number typed on a phone keyboard, which may use a decimal comma. Empty or invalid input gives null.
export function toNumber(value: unknown): number | null {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null
  if (typeof value !== 'string') return null
  const text = value.trim().replace(',', '.')
  if (text === '') return null
  const n = Number(text)
  return Number.isFinite(n) ? n : null
}

const round = (value: number, digits: number) => {
  const f = 10 ** digits
  return Math.round((value + Number.EPSILON) * f) / f
}

// Cost of a charge at a known tariff, in currency units rounded to the cent.
export function costFromTariff(kwh: number | null, pricePerKwh: number | undefined): number | null {
  if (kwh === null || kwh <= 0 || !pricePerKwh || pricePerKwh <= 0) return null
  return round(kwh * pricePerKwh, 2)
}

// Total for a quantity at a unit price (kWh or litres), rounded to the cent.
export function totalFromUnitPrice(quantity: number | null, unitPrice: number | null): number | null {
  if (quantity === null || quantity <= 0 || unitPrice === null || unitPrice < 0) return null
  return round(quantity * unitPrice, 2)
}

// Unit price paid for a total and a quantity, as the text of an input; empty when it cannot be computed.
export function unitPriceText(total: number | null, quantity: number | null, digits: number): string {
  if (total === null || total < 0 || quantity === null || quantity <= 0) return ''
  return String(round(total / quantity, digits))
}

// Price per kWh actually paid, null when it cannot be computed.
export function effectivePricePerKwh(kwh: number | null, cost: number | null): number | null {
  if (kwh === null || kwh <= 0 || cost === null || cost <= 0) return null
  return round(cost / kwh, 4)
}

function pad(n: number) {
  return String(n).padStart(2, '0')
}

// Value of a datetime-local input, in local time.
export function toLocalDateTimeInput(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// Local calendar day (YYYY-MM-DD): the UTC day would be off by one after midnight for a phone in Europe.
export function toLocalDateInput(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export type OdometerCheck = 'none' | 'below' | 'ok'

// Distances are stored in whole km and can come back from a miles field with a fraction: a reading within 1 km of the last one is the same reading.
const ODOMETER_TOLERANCE_KM = 1

/** Compares a typed odometer (km) with the vehicle's last reading: 'below' only when it is lower, 'none' when there is nothing to compare. */
export function checkOdometer(value: unknown, lastReading: number | null | undefined): OdometerCheck {
  const typed = toNumber(value)
  if (typed === null || !lastReading || lastReading <= 0) return 'none'
  return typed < Math.round(lastReading) - ODOMETER_TOLERANCE_KM ? 'below' : 'ok'
}

/** A form date ("YYYY-MM-DD" or "YYYY-MM-DDTHH:mm") as one readable line, "Today" standing in for the current day. */
export function describeQuickDate(value: string, withTime: boolean, now: Date, locale: string, todayLabel: string): string {
  const date = withTime ? new Date(value) : new Date(`${value}T00:00:00`)
  if (Number.isNaN(date.getTime())) return value
  const time = withTime ? date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' }) : ''
  if (toLocalDateInput(date) === toLocalDateInput(now)) return time ? `${todayLabel}, ${time}` : todayLabel
  const day = date.toLocaleDateString(locale, { day: 'numeric', month: 'short', year: 'numeric' })
  return time ? `${day}, ${time}` : day
}

const clean = (v: string) => (v.trim() === '' ? null : v.trim())

export interface ChargeFormValues {
  date: string // datetime-local value
  kwh: string
  cost: string
  address: string
  odometer: string
  notes: string
  documentId: string | null
}

// A manual charge always carries a cost: the API requires it (0 when the charge was free). currency is the
// vehicle's own (the quick sheet has no fx_rate field, so it never enters a foreign one).
export function buildChargePayload(f: ChargeFormValues, currency: string) {
  const kwh = toNumber(f.kwh)
  const cost = toNumber(f.cost)
  if (kwh === null || kwh <= 0) throw new Error(t('quickadd.errors.kwh'))
  if (cost === null || cost < 0) throw new Error(t('quickadd.errors.cost'))
  const odometer = toNumber(f.odometer)
  return {
    date: new Date(f.date).toISOString(),
    kwh_added: kwh,
    cost,
    currency,
    fx_rate: null,
    address: clean(f.address),
    odometer: odometer !== null && odometer > 0 ? odometer : null,
    notes: clean(f.notes),
    document_id: f.documentId || null,
  }
}

export interface FuelFormValues {
  date: string // YYYY-MM-DD
  amount: string
  liters: string
  fullTank: boolean
  odometer: string
  notes: string
}

export function buildFuelPayload(f: FuelFormValues) {
  const amount = toNumber(f.amount)
  const liters = toNumber(f.liters)
  if (amount === null || amount <= 0) throw new Error(t('quickadd.errors.fuelAmount'))
  const odometer = toNumber(f.odometer)
  return {
    date: f.date,
    amount,
    liters: liters !== null && liters > 0 ? liters : undefined,
    odometer: odometer !== null && odometer > 0 ? odometer : undefined,
    is_full_tank: f.fullTank,
    notes: clean(f.notes) ?? undefined,
  }
}

export interface ExpenseFormValues {
  type: ExpenseType
  date: string // datetime-local value
  amount: string
  notes: string
  documentId: string | null
}

export function buildExpensePayload(f: ExpenseFormValues, currency: string) {
  const amount = toNumber(f.amount)
  if (amount === null || amount <= 0) throw new Error(t('quickadd.errors.amount'))
  return {
    type: f.type,
    amount,
    currency,
    fx_rate: null,
    date: new Date(f.date).toISOString(),
    notes: clean(f.notes) ?? '',
    document_id: f.documentId || null,
  }
}

// Minimal view of a TeslaMate charge waiting for its cost.
export interface PendingCharge {
  id: string
  date: string
  kwh_added: number
  address?: string | null
  odometer?: number | null
  notes?: string | null
  document_id?: string | null
}

// The update endpoint validates date and energy, then keeps the TeslaMate values but overwrites notes and
// attachment with what it receives: both are sent back unchanged so completing the cost never wipes them.
export function buildPendingCostPayload(charge: PendingCharge, costText: string, currency: string) {
  const cost = toNumber(costText)
  if (cost === null || cost < 0) throw new Error(t('quickadd.errors.cost'))
  return {
    date: charge.date,
    kwh_added: charge.kwh_added,
    cost,
    currency,
    fx_rate: null,
    address: charge.address ?? null,
    odometer: charge.odometer ?? null,
    notes: charge.notes ?? null,
    document_id: charge.document_id ?? null,
  }
}

function defaultStorage(): StorageLike | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

export function loadMemory(vehicleId: string, storage: StorageLike | null = defaultStorage()): QuickMemory {
  if (!storage) return {}
  try {
    const raw = storage.getItem(MEMORY_PREFIX + vehicleId) ?? storage.getItem(LEGACY_MEMORY_PREFIX + vehicleId)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    const memory: QuickMemory = {}
    if (typeof parsed.pricePerKwh === 'number' && parsed.pricePerKwh > 0) memory.pricePerKwh = parsed.pricePerKwh
    if (typeof parsed.address === 'string' && parsed.address.trim()) memory.address = parsed.address
    return memory
  } catch {
    return {}
  }
}

// Remembers the tariff and place of a saved charge. A free charge (no price) keeps the previous tariff.
export function rememberCharge(
  vehicleId: string,
  entry: { kwh: number; cost: number; address: string | null },
  storage: StorageLike | null = defaultStorage(),
) {
  if (!storage) return
  const memory = loadMemory(vehicleId, storage)
  const price = effectivePricePerKwh(entry.kwh, entry.cost)
  if (price !== null) memory.pricePerKwh = price
  if (entry.address) memory.address = entry.address
  try {
    storage.setItem(MEMORY_PREFIX + vehicleId, JSON.stringify(memory))
  } catch {
    // Storage full or blocked: the memory is a convenience only
  }
}

// api.request answers { queued: true } when the mutation was stored for later because the network is down.
export function isQueued(result: unknown): boolean {
  return typeof result === 'object' && result !== null && (result as { queued?: unknown }).queued === true
}
