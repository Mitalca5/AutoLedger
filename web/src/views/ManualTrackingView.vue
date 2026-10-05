<script setup lang="ts">
import TabBar, { type TabItem } from '@/components/TabBar.vue'
import PageHeader from '@/components/PageHeader.vue'
import { ClipboardList as PageIcon } from 'lucide-vue-next'
import { t } from '@/i18n'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Gauge, Fuel, UploadCloud, Download } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import OdometerReadingsPanel from '@/components/manual/OdometerReadingsPanel.vue'
import FuelLogsPanel from '@/components/manual/FuelLogsPanel.vue'
import CSVImportModal from '@/components/CSVImportModal.vue'
import ExportDataModal from '@/components/ExportDataModal.vue'

type Tab = 'KM' | 'FUEL'

const showImport = ref(false)
const showExport = ref(false)
const reloadKey = ref(0)
// The panels load their data when mounted: remounting them shows what the import added
const onImported = () => {
  reloadKey.value++
}

const route = useRoute()
const router = useRouter()
const vehicleStore = useVehicleStore()

// Readings are shared by every vehicle; fill-ups only exist for combustion vehicles, the energy estimate for electric ones
const tabs = computed<TabItem[]>(() => {
  const list: TabItem[] = [{ key: 'KM', label: t('manual.manualTrackingView.mileage'), icon: Gauge }]
  if (vehicleStore.canRefuel) list.push({ key: 'FUEL', label: t('manual.manualTrackingView.energyTab'), icon: Fuel })
  return list
})

const requested = String(route.query.tab || '').toUpperCase()
const activeTab = ref<Tab>('KM')

function resolveTab(value: string): Tab {
  return ((tabs.value.find((t) => t.key === value)?.key) as Tab | undefined) || 'KM'
}

activeTab.value = resolveTab(requested)

watch(() => route.query.tab, (q) => {
  activeTab.value = resolveTab(String(q || '').toUpperCase())
})

// A tab that does not exist for the newly selected vehicle falls back to the readings
watch(() => vehicleStore.canRefuel, () => {
  activeTab.value = resolveTab(activeTab.value)
})

function select(tab: Tab) {
  activeTab.value = tab
  if (route.query.tab !== tab) router.replace({ query: { ...route.query, tab } })
}
</script>

<template>
  <div class="space-y-5">
    <PageHeader :title="$t('manual.manualTrackingView.manualTracking')" :icon="PageIcon">
      {{ vehicleStore.canRefuel ? $t('manual.manualTrackingView.subtitle', { what: $t('manual.manualTrackingView.fillUps') }) : $t('manual.manualTrackingView.subtitleMileage') }}{{ vehicleStore.activeVehicle ? ` · ${vehicleStore.activeVehicle.name}` : '' }}{{ $t('manual.manualTrackingView.independent') }}
    </PageHeader>

    <div v-if="!vehicleStore.activeVehicle" class="bg-slate-900 border border-slate-800 rounded-2xl p-6 text-sm text-slate-400">
      {{ $t('manual.manualTrackingView.addAVehicleFirst') }}
    </div>

    <template v-else>
      <div class="flex justify-end">
        <div class="flex items-center gap-2">
          <button
            v-if="vehicleStore.canEdit"
            type="button"
            class="tap-text flex items-center gap-1.5 px-3 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 text-xs font-semibold text-slate-200"
            @click="showImport = true"
          >
            <UploadCloud class="w-3.5 h-3.5 text-sky-400" />
            <span>{{ $t('expenses.expensesView.importCsv') }}</span>
          </button>
          <button
            type="button"
            class="tap-text flex items-center gap-1.5 px-3 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 text-xs font-semibold text-slate-200"
            @click="showExport = true"
          >
            <Download class="w-3.5 h-3.5 text-sky-400" />
            <span>{{ $t('import.export') }}</span>
          </button>
        </div>
      </div>
      <TabBar v-if="tabs.length > 1" :model-value="activeTab" :tabs="tabs" :label="$t('manual.manualTrackingView.manualTracking')" id-prefix="manual-tab" @update:model-value="select($event as Tab)" />

      <OdometerReadingsPanel v-if="activeTab === 'KM'" :key="reloadKey" :vehicle="vehicleStore.activeVehicle" :can-edit="vehicleStore.canEdit" />
      <FuelLogsPanel
        v-else-if="activeTab === 'FUEL'"
        :key="reloadKey"
        :vehicle-id="vehicleStore.activeVehicle.id"
        :can-edit="vehicleStore.canEdit"
      />
      <ExportDataModal v-model:open="showExport" :vehicle-id="vehicleStore.activeVehicle.id" />
      <CSVImportModal
        v-model:open="showImport"
        :vehicle-id="vehicleStore.activeVehicle.id"
        :default-type="activeTab === 'FUEL' ? 'FUEL' : 'ODOMETER'"
        @imported="onImported"
      />
    </template>
  </div>
</template>
