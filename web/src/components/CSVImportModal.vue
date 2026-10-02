<script setup lang="ts">
import { ref, watch } from 'vue'
import { computed } from 'vue'
import { currentLocale, t } from '@/i18n'
import { CSV_COLUMNS, csvTemplate, csvTemplateFilename, csvTemplateTypes, type CsvTemplateType } from '@/utils/dataSources'
import { downloadCsv } from '@/utils/csv'
import { api } from '@/services/api'
import type { CSVDateOrder, CSVDecimalSeparator, CSVExecuteResult, CSVImportOptions, CSVImportType, CSVPreviewResult } from '@/services/csvImport'
import { apiErrorMessage } from '@/services/apiError'
import { distanceUnit } from '@/units'
import { useVehicleStore } from '@/stores/vehicle'
import { X, UploadCloud, CheckCircle2, XCircle, ArrowRight } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  vehicleId: string
  defaultType?: CSVImportType
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'imported'): void
}>()

const vehicleStore = useVehicleStore()

const file = ref<File | null>(null)
const selectedType = ref<CSVImportType | ''>(props.defaultType || '')
const skipDuplicates = ref(true)
const dateOrder = ref<CSVDateOrder>('')
const decimalSeparator = ref<CSVDecimalSeparator>('')
const mapping = ref<Record<number, string>>({})
const loading = ref(false)
const error = ref('')

const templateTypes = computed(() => csvTemplateTypes(vehicleStore.isIce ? 'ICE' : 'EV'))

function downloadTemplate(type: CsvTemplateType) {
  const { headers, rows } = csvTemplate(type, currentLocale())
  downloadCsv(csvTemplateFilename(type), headers, rows)
}

const previewResult = ref<CSVPreviewResult | null>(null)
const executeResult = ref<CSVExecuteResult | null>(null)

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) return
    file.value = null
    selectedType.value = props.defaultType || ''
    skipDuplicates.value = true
    dateOrder.value = ''
    decimalSeparator.value = ''
    mapping.value = {}
    loading.value = false
    error.value = ''
    previewResult.value = null
    executeResult.value = null
  },
  { immediate: true },
)

function importOptions(): CSVImportOptions {
  return {
    type: selectedType.value,
    skipDuplicates: skipDuplicates.value,
    mapping: mapping.value,
    dateOrder: dateOrder.value,
    decimalSeparator: decimalSeparator.value,
  }
}

watch([selectedType, skipDuplicates], () => {
  previewResult.value = null
  mapping.value = {}
})

function setColumnField(index: number, field: string) {
  mapping.value = { ...mapping.value, [index]: field }
  void handlePreview()
}

function resetFormat() {
  mapping.value = {}
  dateOrder.value = ''
  decimalSeparator.value = ''
  void handlePreview()
}

const rowErrors = (errors: CSVPreviewResult['errors'] | undefined) =>
  (errors ?? []).map((e) => apiErrorMessage(e, e.message))

function typeLabel(type: CSVPreviewResult['type']): string {
  const keys: Record<string, string> = {
    CHARGES: 'import.typeCharges',
    DRIVES: 'import.typeDrives',
    FUEL: 'import.typeFuel',
    ODOMETER: 'import.typeOdometer',
  }
  return keys[type] ? t(keys[type]) : t('import.detected')
}

function close() {
  emit('update:open', false)
}

function onFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    file.value = target.files[0]
    previewResult.value = null
    executeResult.value = null
    error.value = ''
  }
}

async function handlePreview() {
  const targetVehicleId = props.vehicleId || vehicleStore.activeVehicle?.id
  if (!targetVehicleId || !file.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await api.previewCSVImport(targetVehicleId, file.value, importOptions())
    previewResult.value = res
    if (!selectedType.value && res.type !== 'UNKNOWN') {
      selectedType.value = res.type
    }
  } catch (err: any) {
    error.value = err.message || t('import.previewFailed')
  } finally {
    loading.value = false
  }
}

async function handleExecute() {
  const targetVehicleId = props.vehicleId || vehicleStore.activeVehicle?.id
  if (!targetVehicleId || !file.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await api.executeCSVImport(targetVehicleId, file.value, importOptions())
    executeResult.value = res
    emit('imported')
  } catch (err: any) {
    error.value = err.message || t('import.executeFailed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm" @click.self="close">
    <div class="w-full max-w-2xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
      <!-- Header -->
      <div class="px-6 py-4 border-b border-slate-800 flex items-center justify-between">
        <h3 class="text-lg font-bold text-white flex items-center gap-2">
          <UploadCloud class="w-5 h-5 text-indigo-400" />
          {{ $t('import.modalTitle') }}
        </h3>
        <button @click="close" class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-6 space-y-5 overflow-y-auto">
        <div v-if="error" class="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400">
          {{ error }}
        </div>

        <!-- Result -->
        <div
          v-if="executeResult"
          class="p-4 rounded-2xl space-y-3 border"
          :class="executeResult.committed ? 'bg-emerald-500/10 border-emerald-500/30' : 'bg-rose-500/10 border-rose-500/30'"
        >
          <div class="flex items-center gap-2.5 font-bold" :class="executeResult.committed ? 'text-emerald-400' : 'text-rose-400'">
            <CheckCircle2 v-if="executeResult.committed" class="w-5 h-5" />
            <XCircle v-else class="w-5 h-5" />
            <span>{{ executeResult.committed ? $t('import.successTitle') : $t('import.cancelledTitle') }}</span>
          </div>
          <p v-if="!executeResult.committed" class="text-xs text-slate-300">{{ $t('import.cancelledHint') }}</p>
          <div class="grid grid-cols-3 gap-2 text-center pt-2">
            <div class="p-3 bg-slate-800/80 rounded-xl border border-slate-700/50">
              <span class="text-xs text-slate-400 block">{{ $t('import.imported') }}</span>
              <span class="text-xl font-bold text-emerald-400">{{ executeResult.imported_count }}</span>
            </div>
            <div class="p-3 bg-slate-800/80 rounded-xl border border-slate-700/50">
              <span class="text-xs text-slate-400 block">{{ $t('import.skipped') }}</span>
              <span class="text-xl font-bold text-amber-400">{{ executeResult.skipped_count }}</span>
            </div>
            <div class="p-3 bg-slate-800/80 rounded-xl border border-slate-700/50">
              <span class="text-xs text-slate-400 block">{{ $t('import.errors') }}</span>
              <span class="text-xl font-bold text-rose-400">{{ executeResult.error_count }}</span>
            </div>
          </div>
          <div v-if="executeResult.errors?.length" class="text-xs text-rose-400 space-y-1 max-h-32 overflow-y-auto pt-2">
            <div v-for="(msg, idx) in rowErrors(executeResult.errors)" :key="idx">• {{ msg }}</div>
            <div v-if="executeResult.errors_truncated" class="text-slate-400">{{ $t('import.moreErrors') }}</div>
          </div>
          <div class="pt-2 flex justify-end">
            <button
              @click="close"
              class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-xl transition-colors"
            >
              {{ $t('common.close') }}
            </button>
          </div>
        </div>

        <div v-else class="space-y-4">
          <details class="rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2 text-xs text-slate-300">
            <summary class="cursor-pointer font-semibold text-slate-200">{{ $t('import.helpTitle') }}</summary>
            <p class="mt-2 text-slate-400">{{ $t('import.helpIntro') }}</p>
            <div v-for="type in templateTypes" :key="type" class="mt-3 space-y-1">
              <div class="flex items-center justify-between gap-2">
                <span class="font-semibold text-white">{{ $t(`import.type${type.charAt(0)}${type.slice(1).toLowerCase()}`) }}</span>
                <button type="button" @click="downloadTemplate(type)" class="shrink-0 text-rose-400 hover:text-rose-300 font-semibold">{{ $t('import.downloadTemplate') }}</button>
              </div>
              <p class="break-words text-slate-400">
                <code class="text-emerald-300">{{ CSV_COLUMNS[type].required.join(', ') }}</code>
                <span v-if="CSV_COLUMNS[type].optional.length"> + <code>{{ CSV_COLUMNS[type].optional.join(', ') }}</code></span>
              </p>
            </div>
          </details>

          <!-- File selection -->
          <div>
            <label for="csv-file-input" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">
              {{ $t('import.selectFile') }}
            </label>
            <div class="flex items-center gap-3">
              <input
                id="csv-file-input"
                type="file"
                accept=".csv,text/csv"
                @change="onFileChange"
                class="block w-full text-xs text-slate-400 file:mr-4 file:py-2.5 file:px-4 file:rounded-xl file:border-0 file:text-xs file:font-semibold file:bg-slate-800 file:text-white hover:file:bg-slate-700 cursor-pointer"
              />
              <button
                v-if="file && !previewResult"
                type="button"
                :disabled="loading"
                @click="handlePreview"
                class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl shrink-0 transition-colors disabled:opacity-50"
              >
                {{ loading ? $t('common.loading') : $t('import.previewBtn') }}
              </button>
            </div>
          </div>

          <!-- Type and options -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label for="csv-type-select" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">
                {{ $t('import.typeLabel') }}
              </label>
              <select
                id="csv-type-select"
                v-model="selectedType"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-xs text-white focus:outline-none focus:border-indigo-500"
              >
                <option value="">{{ $t('import.autoDetect') }}</option>
                <option value="CHARGES">{{ $t('import.typeCharges') }}</option>
                <option value="DRIVES">{{ $t('import.typeDrives') }}</option>
                <option value="FUEL">{{ $t('import.typeFuel') }}</option>
                <option value="ODOMETER">{{ $t('import.typeOdometer') }}</option>
              </select>
            </div>
            <div class="flex items-center pt-5">
              <label for="csv-skip-duplicates" class="flex items-center gap-2.5 text-xs text-slate-300 cursor-pointer">
                <input
                  id="csv-skip-duplicates"
                  v-model="skipDuplicates"
                  type="checkbox"
                  class="rounded text-indigo-500 focus:ring-indigo-500/20 bg-slate-900 border-slate-700 w-4 h-4"
                />
                <span>{{ $t('import.skipDuplicates') }}</span>
              </label>
            </div>
          </div>

          <!-- Preview Table -->
          <div v-if="previewResult" class="space-y-3 pt-2">
            <div class="flex items-center justify-between text-xs text-slate-300 border-b border-slate-800 pb-2">
              <span class="font-semibold text-indigo-400">
                {{ typeLabel(previewResult.type) }}
                — {{ previewResult.total_rows }} {{ $t('import.linesFound') }}
              </span>
              <span class="text-slate-400">{{ previewResult.headers.length }} {{ $t('import.columns') }}</span>
            </div>

            <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
              <span class="text-emerald-400">{{ previewResult.valid_rows }} {{ $t('import.validRows') }}</span>
              <span class="text-amber-400">{{ previewResult.duplicate_rows }} {{ $t('import.duplicateRows') }}</span>
              <span class="text-rose-400">{{ previewResult.invalid_rows }} {{ $t('import.invalidRows') }}</span>
            </div>
            <p class="text-[11px] text-slate-500">{{ $t('import.distanceHint', { unit: distanceUnit() }) }}</p>

            <div v-if="previewResult.errors?.length" class="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl text-xs text-rose-400 space-y-1 max-h-32 overflow-y-auto">
              <div v-for="(msg, idx) in rowErrors(previewResult.errors)" :key="idx">• {{ msg }}</div>
              <div v-if="previewResult.errors_truncated" class="text-slate-400">{{ $t('import.moreErrors') }}</div>
            </div>

            <!-- Column mapping -->
            <details class="rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2 text-xs text-slate-300">
              <summary class="cursor-pointer font-semibold text-slate-200">{{ $t('import.mappingTitle') }}</summary>
              <p class="mt-2 text-slate-400">{{ $t('import.mappingHint') }}</p>
              <div class="mt-3 space-y-2">
                <div v-for="col in previewResult.mapping" :key="col.index" class="grid grid-cols-2 gap-2 items-center">
                  <label :for="`csv-col-${col.index}`" class="truncate font-mono text-[11px] text-slate-400" :title="col.header">{{ col.header }}</label>
                  <select
                    :id="`csv-col-${col.index}`"
                    :value="col.field"
                    @change="setColumnField(col.index, ($event.target as HTMLSelectElement).value)"
                    class="min-w-0 w-full bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none focus:border-indigo-500"
                  >
                    <option value="">{{ $t('import.ignoreColumn') }}</option>
                    <option v-if="col.field && !previewResult.fields.includes(col.field)" :value="col.field" disabled>{{ col.field }}</option>
                    <option v-for="f in previewResult.fields" :key="f" :value="f">{{ f }}</option>
                  </select>
                </div>
              </div>
              <div class="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-2">
                <div>
                  <label for="csv-date-order" class="block text-slate-400 mb-1">{{ $t('import.dateOrder') }}</label>
                  <select id="csv-date-order" v-model="dateOrder" @change="handlePreview" class="w-full bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white">
                    <option value="">{{ $t('import.autoDetect') }}</option>
                    <option value="dmy">{{ $t('import.dateDmy') }}</option>
                    <option value="mdy">{{ $t('import.dateMdy') }}</option>
                    <option value="ymd">{{ $t('import.dateYmd') }}</option>
                  </select>
                </div>
                <div>
                  <label for="csv-decimal" class="block text-slate-400 mb-1">{{ $t('import.decimalSeparator') }}</label>
                  <select id="csv-decimal" v-model="decimalSeparator" @change="handlePreview" class="w-full bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs text-white">
                    <option value="">{{ $t('import.autoDetect') }}</option>
                    <option value=".">{{ $t('import.decimalDot') }}</option>
                    <option value=",">{{ $t('import.decimalComma') }}</option>
                  </select>
                </div>
              </div>
              <button type="button" @click="resetFormat" class="mt-3 text-rose-400 hover:text-rose-300 font-semibold">{{ $t('import.resetMapping') }}</button>
            </details>

            <!-- Sample rows -->
            <div class="overflow-x-auto border border-slate-800 rounded-xl">
              <table class="w-full text-left text-[11px] text-slate-300">
                <thead class="bg-slate-800/80 text-slate-400 font-semibold border-b border-slate-700/60">
                  <tr>
                    <th v-for="h in previewResult.headers" :key="h" class="px-3 py-2 whitespace-nowrap">{{ h }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/60">
                  <tr v-for="(row, idx) in previewResult.sample_rows" :key="idx" class="hover:bg-slate-800/40">
                    <td v-for="h in previewResult.headers" :key="h" class="px-3 py-2 whitespace-nowrap font-mono text-[10px]">
                      {{ row[h] }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Confirm Import Action -->
            <div class="pt-3 flex justify-end gap-3">
              <button
                type="button"
                @click="close"
                class="px-4 py-2 rounded-xl border border-slate-700 text-xs font-semibold text-slate-300 hover:bg-slate-800 transition-colors"
              >
                {{ $t('common.cancel') }}
              </button>
              <button
                type="button"
                :disabled="loading || previewResult.invalid_rows > 0 || previewResult.valid_rows === 0"
                :title="previewResult.invalid_rows > 0 ? $t('import.fixFirst') : undefined"
                @click="handleExecute"
                class="px-5 py-2 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white text-xs font-semibold rounded-xl shadow-lg shadow-indigo-600/25 transition-all flex items-center gap-1.5 disabled:opacity-50"
              >
                <span>{{ loading ? $t('import.importing') : $t('import.confirmImport') }}</span>
                <ArrowRight class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
