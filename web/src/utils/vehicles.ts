import { t } from '@/i18n'
import { todayIso } from '@/utils/dates'
import { formatAmount } from '@/currency'

export const acquisitionLabel = (type: string): string => t(`vehicles.acquisition.${type}`)

/** Role of a user on a vehicle, in the current language. */
export const roleLabel = (role: string): string =>
  role === 'OWNER' ? t('vehicles.roles.OWNER') : role === 'EDITOR' ? t('vehicles.roles.EDITOR') : t('vehicles.roles.VIEWER')

export const ownershipSteps = () => [
  { step: 1, title: t('vehicles.wizardSteps.one.title'), description: t('vehicles.wizardSteps.one.description') },
  { step: 2, title: t('vehicles.wizardSteps.two.title'), description: t('vehicles.wizardSteps.two.description') },
  { step: 3, title: t('vehicles.wizardSteps.three.title'), description: t('vehicles.wizardSteps.three.description') },
]

export function emptyOwnership() {
  return {
    acquisition_type: 'CASH',
    start_date: todayIso(),
    start_odometer: null as number | null,
    purchase_price: null as number | null,
    purchase_fees: null as number | null,
    incentives: null as number | null,
    expected_resale_value: null as number | null,
    expected_holding_months: null as number | null,
    loan_amount: null as number | null,
    loan_rate_pct: null as number | null,
    loan_duration_months: null as number | null,
    loan_fees: null as number | null,
    loan_insurance_monthly: null as number | null,
    lease_down_payment: null as number | null,
    lease_monthly_rent: null as number | null,
    lease_duration_months: null as number | null,
    lease_fees: null as number | null,
    lease_deposit: null as number | null,
    lease_km_allowance_per_year: null as number | null,
    lease_excess_km_price: null as number | null,
    lease_end_fees_estimate: null as number | null,
    lease_purchase_option_price: null as number | null,
    lease_includes_maintenance: false,
    lease_includes_insurance: false,
    lease_includes_tires: false,
    option_exercised_date: '',
    end_date: '',
    sale_price: null as number | null,
  }
}

export type OwnershipForm = ReturnType<typeof emptyOwnership>

// What a powertrain can record. Components and stores ask for a capability instead of comparing the powertrain.
type PowertrainLike = string | null | undefined

/** A battery to plug in: the vehicle has charging sessions. */
export function canCharge(powertrain: PowertrainLike): boolean {
  return powertrain !== 'ICE'
}

/** Fuel burned: the vehicle has fill-ups. */
export function canRefuel(powertrain: PowertrainLike): boolean {
  return powertrain === 'ICE'
}

/** A teslamateapi connection makes sense for the powertrain. */
export function canLinkTeslaMate(powertrain: PowertrainLike): boolean {
  return canCharge(powertrain)
}

/** TeslaMate features (synchronization, drives, battery and temperature data) apply to an electric vehicle with a teslamateapi URL. */
export function hasTeslaMate(v: { powertrain?: string; teslamate_api_url?: string | null } | null | undefined): boolean {
  return !!v && canLinkTeslaMate(v.powertrain) && !!v.teslamate_api_url?.trim()
}

/**
 * Computes the next auto-incremental TeslaMate car ID based on existing vehicles (defaults to 1).
 */
export function nextTeslaMateCarId(vehicles?: { teslamate_car_id?: number | null }[] | null): number {
  if (!vehicles || vehicles.length === 0) return 1
  const ids = vehicles
    .map((v) => v.teslamate_car_id)
    .filter((id): id is number => typeof id === 'number' && Number.isFinite(id) && id > 0)
  if (ids.length === 0) return 1
  return Math.max(...ids) + 1
}

export function emptyVehicleForm(existingVehicles?: { teslamate_car_id?: number | null }[] | null) {
  return {
    name: '',
    powertrain: 'EV',
    make: '',
    model: '',
    currency: 'EUR',
    vin: '',
    current_odometer: 0,
    teslamate_car_id: nextTeslaMateCarId(existingVehicles) as number | null,
    teslamate_api_url: '',
    teslamate_grafana_url: '',
    teslamate_auth_type: 'NONE',
    teslamate_api_key: '',
    teslamate_basic_user: '',
    teslamate_basic_pass: '',
    estimated_kwh_100km: null as number | null,
    estimated_price_per_kwh: null as number | null,
    tariff_plan_id: null as string | null,
    default_driver_id: null as string | null,
    is_home_charger_default: false,
  }
}

export type VehicleForm = ReturnType<typeof emptyVehicleForm>

/** Form values of an existing vehicle; secrets are never sent back by the API so they start empty. */
export function vehicleFormFrom(v: any): VehicleForm {
  return {
    name: v.name,
    powertrain: v.powertrain || 'EV',
    make: v.make || '',
    model: v.model || '',
    currency: v.currency || 'EUR',
    vin: v.vin || '',
    current_odometer: v.current_odometer ? Math.round(v.current_odometer) : 0,
    teslamate_car_id: v.teslamate_car_id || 1,
    teslamate_api_url: v.teslamate_api_url || '',
    teslamate_grafana_url: v.teslamate_grafana_url || '',
    teslamate_auth_type: v.teslamate_auth_type || 'NONE',
    teslamate_api_key: '',
    teslamate_basic_user: v.teslamate_basic_user || '',
    teslamate_basic_pass: '',
    estimated_kwh_100km: v.estimated_kwh_100km ?? null,
    estimated_price_per_kwh: v.estimated_price_per_kwh ?? null,
    tariff_plan_id: v.tariff_plan_id || null,
    default_driver_id: v.default_driver_id || null,
    is_home_charger_default: !!v.is_home_charger_default,
  }
}

// Empty numeric inputs are sent as null, never as ""
export function nullIfEmpty(v: any) {
  return v === '' || v === undefined ? null : v
}

export function toDateInput(v?: string | null) {
  return v ? new Date(v).toISOString().substring(0, 10) : ''
}

/** Form for the contract of a vehicle: its saved contract, or a new one starting at the vehicle's odometer. */
export function ownershipFormFrom(o: any | null | undefined, vehicle: any): OwnershipForm {
  return o
    ? {
        ...emptyOwnership(),
        ...o,
        start_date: toDateInput(o.start_date),
        option_exercised_date: toDateInput(o.option_exercised_date),
        end_date: toDateInput(o.end_date),
      }
    : { ...emptyOwnership(), start_odometer: vehicle.current_odometer ? Math.round(vehicle.current_odometer) : null }
}

/** The payload of a contract: empty numeric fields become null. */
export function ownershipPayload(form: OwnershipForm) {
  const f: any = { ...form }
  for (const key of Object.keys(f)) {
    if (typeof f[key] !== 'boolean') f[key] = nullIfEmpty(f[key])
  }
  return f
}

export const isLeaseType = (f: { acquisition_type: string }) => ['LOA', 'LLD'].includes(f.acquisition_type)
export const isPurchaseType = (f: { acquisition_type: string }) => ['CASH', 'LOAN'].includes(f.acquisition_type)
export const isOwnedPhase = (f: { acquisition_type: string; option_exercised_date: string }) =>
  isPurchaseType(f) || (f.acquisition_type === 'LOA' && !!f.option_exercised_date)

/** Loan annuity preview; null until the amount and the duration are known. */
export function loanPreview(f: OwnershipForm) {
  const p = Number(f.loan_amount) || 0
  const n = Number(f.loan_duration_months) || 0
  const r = (Number(f.loan_rate_pct) || 0) / 1200
  if (!p || !n) return null
  const payment = r === 0 ? p / n : (p * r) / (1 - Math.pow(1 + r, -n))
  const insurance = Number(f.loan_insurance_monthly) || 0
  return { payment, totalInterest: payment * n - p, totalCost: payment * n - p + insurance * n + (Number(f.loan_fees) || 0) }
}

/** Lease total preview (cash paid over the contract, purchase option excluded). */
export function leasePreview(f: OwnershipForm) {
  const rent = Number(f.lease_monthly_rent) || 0
  const n = Number(f.lease_duration_months) || 0
  if (!rent || !n) return null
  const total = rent * n + (Number(f.lease_down_payment) || 0) + (Number(f.lease_fees) || 0) + (Number(f.lease_end_fees_estimate) || 0)
  const allowance = Number(f.lease_km_allowance_per_year) || 0
  return { total, perMonth: total / n, totalKm: (allowance * n) / 12 }
}

export function ownershipSummary(o: any, currency = 'EUR') {
  if (!o) return null
  const fmt = (v: number) => formatAmount(Number(v), currency, 0)
  if (o.acquisition_type === 'CASH' || o.acquisition_type === 'LOAN') {
    return `${acquisitionLabel(o.acquisition_type)} • ${fmt(o.purchase_price)}`
  }
  return `${o.acquisition_type} • ${t('vehicles.leaseSummary', { rent: fmt(o.lease_monthly_rent), months: o.lease_duration_months })}`
}

/** The message to show when a wizard step is incomplete, or null when it can be left. */
export function ownershipStepError(f: OwnershipForm, step: number): string | null {
  if (step === 1) {
    if (!f.acquisition_type) return t('vehicles.wizardErrors.mode')
    if (!f.start_date) return t('vehicles.wizardErrors.startDate')
    return null
  }
  if (step === 2) {
    if (isPurchaseType(f)) {
      if (f.purchase_price === null || f.purchase_price === undefined || Number(f.purchase_price) <= 0) {
        return t('vehicles.wizardErrors.price')
      }
    }
    if (f.acquisition_type === 'LOAN') {
      if (!f.loan_amount || Number(f.loan_amount) <= 0) return t('vehicles.wizardErrors.loanAmount')
      if (f.loan_duration_months === null || Number(f.loan_duration_months) <= 0) return t('vehicles.wizardErrors.loanDuration')
    }
    if (isLeaseType(f)) {
      if (f.lease_monthly_rent === null || f.lease_monthly_rent === undefined || Number(f.lease_monthly_rent) < 0) return t('vehicles.wizardErrors.rent')
      if (!f.lease_duration_months || Number(f.lease_duration_months) <= 0) return t('vehicles.wizardErrors.leaseDuration')
    }
    return null
  }
  return null
}
