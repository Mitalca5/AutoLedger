<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import { LayoutDashboard as PageIcon } from 'lucide-vue-next'
import { ref, onMounted, watch, computed } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { usePreferencesStore } from '@/stores/preferences'
import { api, type MaintenanceReminder } from '@/services/api'

import ResidualValuePanel from '@/components/dashboard/ResidualValuePanel.vue'
import EnergyEfficiencyPanel from '@/components/dashboard/EnergyEfficiencyPanel.vue'
import UrgentRemindersBanner from '@/components/dashboard/UrgentRemindersBanner.vue'
import NextDueCard from '@/components/dashboard/NextDueCard.vue'
import DataQualityCard from '@/components/dashboard/DataQualityCard.vue'
import LeaseContractCard from '@/components/dashboard/LeaseContractCard.vue'
import TcoMetricsGrid from '@/components/dashboard/TcoMetricsGrid.vue'
import CurrentMonthBanner from '@/components/dashboard/CurrentMonthBanner.vue'
import MonthlyCostChart from '@/components/dashboard/MonthlyCostChart.vue'
import CostBreakdownDonut from '@/components/dashboard/CostBreakdownDonut.vue'
import MileageCostChart from '@/components/dashboard/MileageCostChart.vue'
import TagBreakdown from '@/components/dashboard/TagBreakdown.vue'
import MonthDetailModal from '@/components/dashboard/MonthDetailModal.vue'
import MonthlyRangeSelector from '@/components/dashboard/MonthlyRangeSelector.vue'
import { distanceUnit } from '@/units'

// The page loads the TCO and the reminders; each card and chart of the dashboard is a component that
// receives the data it shows. A click on a month (chart or banner) opens its detail modal.
const vehicleStore = useVehicleStore()
const prefs = usePreferencesStore()
const vehicleId = computed(() => vehicleStore.activeVehicle?.id ?? '')
const tco = ref<any | null>(null)
const loading = ref(true)
const dashboardReminders = ref<MaintenanceReminder[]>([])
const selectedMonth = ref<any | null>(null)

// Monthly costs for the charts; null until the TCO is loaded
const monthlyCosts = computed(() => (tco.value ? tco.value.monthly_costs || [] : null))

function openMonthDetail(m: any) {
  if (!m) return
  selectedMonth.value = m
}

async function loadTCO() {
  if (!vehicleStore.activeVehicle) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const [tcoData, remindersData] = await Promise.all([
      api.getTCO(vehicleStore.activeVehicle.id),
      api.getReminders(vehicleStore.activeVehicle.id).catch(() => []),
    ])
    tco.value = tcoData
    dashboardReminders.value = remindersData
  } catch (err) {
    console.error('Failed to load TCO or reminders', err)
  } finally {
    loading.value = false
  }
}

watch(
  () => [vehicleStore.activeVehicle?.id, vehicleStore.lastSyncTimestamp],
  () => {
    loadTCO()
  }
)

onMounted(() => {
  loadTCO()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header Summary -->
    <PageHeader :title="$t('dashboard.dashboardView.tcoDashboard')" :icon="PageIcon">
      {{ $t('dashboard.dashboardView.subtitle', { unit: distanceUnit(), name: vehicleStore.activeVehicle?.name || $t('dashboard.dashboardView.yourVehicle') }) }}
      <template #actions>
      <div class="flex items-center gap-2">
        <router-link
          to="/expenses"
          class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 transition-colors"
        >
          {{ $t('dashboard.dashboardView.expense') }}
        </router-link>
        <router-link
          to="/drives"
          class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 transition-colors"
        >
          {{ $t('dashboard.dashboardView.viewDrives') }}
        </router-link>
      </div>
    
      </template>
    </PageHeader>

    <!-- Empty state if no vehicle -->
    <div v-if="!vehicleStore.activeVehicle" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl">
      <p class="text-slate-400 mb-4">{{ $t('dashboard.dashboardView.noVehicleConfigured') }}</p>
      <router-link to="/vehicles" class="px-4 py-2 bg-rose-600 text-white text-sm font-semibold rounded-xl">
        {{ $t('dashboard.dashboardView.createYourFirstVehicle') }}
      </router-link>
    </div>

    <!-- SKELETON LOADING STATE -->
    <div v-else-if="loading" class="space-y-6 animate-pulse">
      <!-- 4 KPI Card Skeletons -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <div v-for="i in 4" :key="i" class="bg-slate-900 border border-slate-800 p-5 rounded-2xl h-32 flex flex-col justify-between">
          <div class="flex justify-between items-center">
            <div class="h-3 w-20 bg-slate-800 rounded"></div>
            <div class="h-8 w-8 bg-slate-800 rounded-xl"></div>
          </div>
          <div class="space-y-2">
            <div class="h-7 w-28 bg-slate-800 rounded"></div>
            <div class="h-3 w-36 bg-slate-800/60 rounded"></div>
          </div>
        </div>
      </div>

      <!-- Charts Skeletons -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div class="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-5 h-80 flex flex-col justify-between">
          <div class="h-4 w-44 bg-slate-800 rounded"></div>
          <div class="h-56 w-full bg-slate-800/40 rounded-xl"></div>
        </div>
        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 h-80 flex flex-col justify-between">
          <div class="h-4 w-32 bg-slate-800 rounded"></div>
          <div class="h-52 w-52 rounded-full border-8 border-slate-800 mx-auto"></div>
        </div>
      </div>

      <!-- Second Chart Skeleton -->
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 h-80 flex flex-col justify-between">
        <div class="h-4 w-52 bg-slate-800 rounded"></div>
        <div class="h-56 w-full bg-slate-800/40 rounded-xl"></div>
      </div>
    </div>

    <!-- REAL CONTENT WHEN LOADED -->
    <div v-else class="space-y-4 sm:space-y-6">
      <UrgentRemindersBanner :reminders="dashboardReminders" />

      <TcoMetricsGrid :tco="tco" />

      <LeaseContractCard :tco="tco" />

      <DataQualityCard :tco="tco" :vehicle-id="vehicleId" />

      <NextDueCard :reminders="dashboardReminders" :can-edit="vehicleStore.canEdit" />

      <CurrentMonthBanner :monthly-costs="tco?.monthly_costs" @open-month="openMonthDetail" />

      <!-- One period for the energy, monthly cost and mileage sections below -->
      <div class="sticky top-[4.25rem] z-30 -mx-1 flex flex-wrap items-center justify-between gap-2 rounded-2xl border border-slate-800 bg-slate-900/95 px-4 py-2 backdrop-blur-md">
        <span class="text-xs font-semibold text-slate-300">{{ $t('dashboard.dashboardView.chartsPeriod') }}</span>
        <MonthlyRangeSelector
          :model-value="prefs.dashboardRange"
          :label="$t('dashboard.dashboardView.chartsPeriod')"
          @update:model-value="prefs.setDashboardRange"
        />
      </div>

      <!-- Energy efficiency: consumption, real cost per 100 km and charging habits (electric vehicles) -->
      <EnergyEfficiencyPanel
        v-if="vehicleStore.activeVehicle && vehicleStore.canCharge && (!vehicleStore.canRefuel || (tco?.total_kwh_added || 0) > 0)"
        :vehicle-id="vehicleStore.activeVehicle.id"
        :grafana-url="vehicleStore.activeVehicle.teslamate_grafana_url"
        :sync-key="vehicleStore.lastSyncTimestamp"
        show-link
      />

      <ResidualValuePanel
        v-if="vehicleStore.activeVehicle"
        :vehicle-id="vehicleStore.activeVehicle.id"
        :show-battery="vehicleStore.canCharge"
      />

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <MonthlyCostChart :monthly-costs="monthlyCosts" @open-month="openMonthDetail" />
        <CostBreakdownDonut :tco="tco" />
      </div>

      <MileageCostChart :monthly-costs="monthlyCosts" @open-month="openMonthDetail" />

      <TagBreakdown v-if="prefs.proPersoEnabled" :tco="tco" />
    </div>

    <MonthDetailModal v-model:month="selectedMonth" :monthly-costs="tco?.monthly_costs || []" />
  </div>
</template>
