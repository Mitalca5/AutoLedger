<script setup lang="ts">
import { Zap, Disc, Wrench, Shield } from 'lucide-vue-next'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistance, perDistance } from '@/units'
import CostItemRow from '@/components/costs/CostItemRow.vue'

// The energy, tires, maintenance and insurance lines of the cost breakdown of a drive or a trip
defineProps<{ drive: any; breakdown: any; currency: string }>()
</script>

<template>
  <!-- 1. Électricité -->
  <CostItemRow
    :icon="Zap"
    tone="sky"
    :label="$t('drives.driveCostModal.electricEnergy')"
    :sub="$t('drives.driveCostModal.kwhKwh', { electricity_kwh: drive.costs?.electricity_kwh || 0, electricity_rate: formatAmount(drive.costs?.electricity_rate || 0.22, currency, 3) })"
    :amount="drive.costs?.electricity_cost || 0"
    :share-pct="breakdown.byKey.energy.sharePct"
    :cost-per-km="breakdown.byKey.energy.costPerKm"
    :currency="currency"
  >
    <template #badge>
      <span v-if="drive.costs?.energy_source === 'DEFAULT' || drive.costs?.electricity_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium">{{ $t('drives.driveCostModal.estimate') }}</span>
    </template>
  </CostItemRow>

  <!-- 2. Pneus -->
  <CostItemRow
    :icon="Disc"
    tone="emerald"
    :label="$t('drives.driveCostModal.tireWear')"
    :sub="`${formatDistance(drive.distance_km, 1)} × ${formatAmount(perDistance(drive.costs?.tires_rate || 0.02), currency, 3)}/${distanceUnit()}`"
    :amount="drive.costs?.tires_cost || 0"
    :share-pct="breakdown.byKey.tires.sharePct"
    :cost-per-km="breakdown.byKey.tires.costPerKm"
    :currency="currency"
  >
    <template #badge>
      <span v-if="drive.costs?.tires_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium">{{ $t('drives.driveCostModal.estimate') }}</span>
      <span v-else-if="drive.costs?.tires_rate_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-success-500/10 text-success-400 font-medium">{{ $t('drives.driveCostModal.includedInTheLease2') }}</span>
    </template>
  </CostItemRow>

  <!-- 3. Entretien -->
  <CostItemRow
    :icon="Wrench"
    tone="pink"
    :label="$t('drives.driveCostModal.maintenanceProvision')"
    :sub="`${formatDistance(drive.distance_km, 1)} × ${formatAmount(perDistance(drive.costs?.maintenance_rate || 0.015), currency, 3)}/${distanceUnit()}`"
    :amount="drive.costs?.maintenance_cost || 0"
    :share-pct="breakdown.byKey.maintenance.sharePct"
    :cost-per-km="breakdown.byKey.maintenance.costPerKm"
    :currency="currency"
  >
    <template #badge>
      <span v-if="drive.costs?.maintenance_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium">{{ $t('drives.driveCostModal.estimate') }}</span>
      <span v-else-if="drive.costs?.maintenance_rate_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-success-500/10 text-success-400 font-medium">{{ $t('drives.driveCostModal.includedInTheLease2') }}</span>
    </template>
  </CostItemRow>

  <!-- 4. Assurance -->
  <CostItemRow
    :icon="Shield"
    tone="purple"
    :label="$t('drives.driveCostModal.insuranceShareFixedCost')"
    :sub="`${formatDistance(drive.distance_km, 1)} × ${formatAmount(perDistance(drive.costs?.insurance_rate || 0), currency, 3)}/${distanceUnit()}`"
    :amount="drive.costs?.insurance_cost || 0"
    :share-pct="breakdown.byKey.insurance.sharePct"
    :cost-per-km="breakdown.byKey.insurance.costPerKm"
    :currency="currency"
  >
    <template #badge>
      <span
        v-if="drive.costs?.insurance_source === 'RECORDED_EXPENSES'"
        class="text-[9px] px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 font-medium"
        :title="$t('drives.driveCostModal.premiumsPaidOverTheLast')"
      >
        {{ $t('drives.driveCostModal.actualPremiums') }}
      </span>
      <span v-else-if="drive.costs?.insurance_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-success-500/10 text-success-400 font-medium">
        {{ $t('drives.driveCostModal.includedInTheLease') }}
      </span>
      <span
        v-else-if="drive.costs?.insurance_source === 'INSUFFICIENT_DISTANCE'"
        class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium"
        :title="$t('drives.driveCostModal.lessThan500KmDriven', { min: formatDistance(500) })"
      >
        {{ $t('drives.driveCostModal.notEnoughKm') }}
      </span>
      <span v-else class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium" :title="$t('drives.driveCostModal.noInsurancePremiumRecordedIn')">
        {{ $t('drives.driveCostModal.notEntered') }}
      </span>
    </template>
  </CostItemRow>
</template>
