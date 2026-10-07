<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import { ClipboardList as PageIcon } from 'lucide-vue-next'
import { ref } from 'vue'
import { UploadCloud, Download } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import OdometerReadingsPanel from '@/components/manual/OdometerReadingsPanel.vue'
import CSVImportModal from '@/components/CSVImportModal.vue'
import ExportDataModal from '@/components/ExportDataModal.vue'

const showImport = ref(false)
const showExport = ref(false)
const reloadKey = ref(0)
// The panel loads its data when mounted: remounting it shows what the import added
const onImported = () => {
  reloadKey.value++
}

const vehicleStore = useVehicleStore()
</script>

<template>
  <div class="space-y-5">
    <PageHeader :title="$t('manual.manualTrackingView.manualTracking')" :icon="PageIcon">
      {{ $t('manual.manualTrackingView.subtitleMileage') }}{{ vehicleStore.activeVehicle ? ` · ${vehicleStore.activeVehicle.name}` : '' }}{{ $t('manual.manualTrackingView.independent') }}
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
            class="btn btn-secondary tap-text"
            @click="showImport = true"
          >
            <UploadCloud class="w-3.5 h-3.5 text-sky-400" />
            <span>{{ $t('expenses.expensesView.importCsv') }}</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary tap-text"
            @click="showExport = true"
          >
            <Download class="w-3.5 h-3.5 text-sky-400" />
            <span>{{ $t('import.export') }}</span>
          </button>
        </div>
      </div>

      <OdometerReadingsPanel :key="reloadKey" :vehicle="vehicleStore.activeVehicle" :can-edit="vehicleStore.canEdit" />
      <ExportDataModal v-model:open="showExport" :vehicle-id="vehicleStore.activeVehicle.id" />
      <CSVImportModal
        v-model:open="showImport"
        :vehicle-id="vehicleStore.activeVehicle.id"
        default-type="ODOMETER"
        @imported="onImported"
      />
    </template>
  </div>
</template>
