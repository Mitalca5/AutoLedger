<script setup lang="ts">
import { t } from '@/i18n'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ClipboardList, Gauge, Fuel, Zap, UploadCloud, Download } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import OdometerReadingsPanel from '@/components/manual/OdometerReadingsPanel.vue'
import FuelLogsPanel from '@/components/manual/FuelLogsPanel.vue'
import CSVImportModal from '@/components/CSVImportModal.vue'
import ExportDataModal from '@/components/ExportDataModal.vue'
import EstimatedEnergyPanel from '@/components/manual/EstimatedEnergyPanel.vue'

type Tab = 'KM' | 'FUEL' | 'ENERGY'

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
const tabs = computed<{ key: Tab; label: string; icon: any }[]>(() => {
  const list: { key: Tab; label: string; icon: any }[] = [{ key: 'KM', label: t('manual.manualTrackingView.mileage'), icon: Gauge }]
  if (vehicleStore.canRefuel) list.push({ key: 'FUEL', label: t('manual.manualTrackingView.fillUpsTab'), icon: Fuel })
  else list.push({ key: 'ENERGY', label: t('manual.manualTrackingView.estimatedEnergy'), icon: Zap })
  return list
})

const requested = String(route.query.tab || '').toUpperCase()
const activeTab = ref<Tab>('KM')

function resolveTab(value: string): Tab {
  return (tabs.value.find((t) => t.key === value)?.key) || 'KM'
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
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center">
        <ClipboardList class="w-5 h-5 text-cyan-400" />
      </div>
      <div>
        <h2 class="text-xl font-bold text-white">{{ $t('manual.manualTrackingView.manualTracking') }}</h2>
        <p class="text-xs text-slate-400">
          {{ $t('manual.manualTrackingView.subtitle', { what: vehicleStore.canRefuel ? $t('manual.manualTrackingView.fillUps') : $t('manual.manualTrackingView.energy') }) }}{{ vehicleStore.activeVehicle ? ` · ${vehicleStore.activeVehicle.name}` : '' }}{{ $t('manual.manualTrackingView.independent') }}
        </p>
      </div>
    </div>

    <div v-if="!vehicleStore.activeVehicle" class="bg-slate-900 border border-slate-800 rounded-2xl p-6 text-sm text-slate-400">
      {{ $t('manual.manualTrackingView.addAVehicleFirst') }}
    </div>

    <template v-else>
      <div class="flex items-center bg-slate-900 border border-slate-800 rounded-2xl p-1 gap-1 w-fit max-w-full overflow-x-auto" role="tablist">
        <button
          v-for="t in tabs"
          :key="t.key"
          type="button"
          role="tab"
          :aria-selected="activeTab === t.key"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors shrink-0"
          :class="activeTab === t.key ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 shadow-sm' : 'text-slate-400 hover:text-white'"
          @click="select(t.key)"
        >
          <component :is="t.icon" class="w-3.5 h-3.5" />
          <span>{{ t.label }}</span>
        </button>
        <button
          v-if="vehicleStore.canEdit && activeTab !== 'ENERGY'"
          type="button"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold text-slate-300 hover:text-white shrink-0"
          @click="showImport = true"
        >
          <UploadCloud class="w-3.5 h-3.5 text-sky-400" />
          <span>{{ $t('expenses.expensesView.importCsv') }}</span>
        </button>
        <button
          type="button"
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold text-slate-300 hover:text-white shrink-0"
          @click="showExport = true"
        >
          <Download class="w-3.5 h-3.5 text-sky-400" />
          <span>{{ $t('import.export') }}</span>
        </button>
      </div>

      <OdometerReadingsPanel v-if="activeTab === 'KM'" :key="reloadKey" :vehicle="vehicleStore.activeVehicle" :can-edit="vehicleStore.canEdit" />
      <FuelLogsPanel
        v-else-if="activeTab === 'FUEL'"
        :key="reloadKey"
        :vehicle-id="vehicleStore.activeVehicle.id"
        :can-edit="vehicleStore.canEdit"
      />
      <EstimatedEnergyPanel v-else-if="activeTab === 'ENERGY'" :vehicle="vehicleStore.activeVehicle" :can-edit="vehicleStore.canEdit" />
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
