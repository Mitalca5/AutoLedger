<script setup lang="ts">
import { ref, watch } from 'vue'
import { computed } from 'vue'
import { currentLocale, intlLocale, t } from '@/i18n'
import { CSV_COLUMNS, csvTemplate, csvTemplateFilename, csvTemplateTypes, type CsvTemplateType } from '@/utils/dataSources'
import { downloadCsv } from '@/utils/csv'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { profileColumns, profileMapping, type CSVDateOrder, type CSVDecimalSeparator, type CSVExecuteResult, type CSVImportBatch, type CSVImportOptions, type CSVImportProfile, type CSVImportType, type CSVPreviewResult } from '@/services/csvImport'
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
const { showConfirm } = useConfirm()

const file = ref<File | null>(null)
const selectedType = ref<CSVImportType | ''>(props.defaultType || '')
const skipDuplicates = ref(true)
const dateOrder = ref<CSVDateOrder>('')
const decimalSeparator = ref<CSVDecimalSeparator>('')
const mapping = ref<Record<number, string>>({})
const profiles = ref<CSVImportProfile[]>([])
const profileName = ref('')
const loading = ref(false)
const error = ref('')

const templateTypes = computed(() => csvTemplateTypes(vehicleStore.activeVehicle?.powertrain))

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
    profileName.value = ''
    void loadProfiles()
    void loadBatches()
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

function onOptionsChange() {
  previewResult.value = null
  mapping.value = {}
}

function setColumnField(index: number, field: string) {
  mapping.value = { ...mapping.value, [index]: field }
  void handlePreview()
}

const batches = ref<CSVImportBatch[]>([])

async function loadBatches() {
  try {
    batches.value = await api.listImportBatches(props.vehicleId)
  } catch {
    batches.value = []
  }
}

async function undoBatch(batch: CSVImportBatch) {
  const ok = await showConfirm({
    title: t('import.undoTitle'),
    message: t('import.undoMessage', { count: batch.remaining }),
    confirmText: t('import.undo'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.undoImportBatch(props.vehicleId, batch.id)
    emit('imported')
  } catch (err: any) {
    error.value = err.message || t('import.undoFailed')
  }
  await loadBatches()
}

async function loadProfiles() {
  try {
    profiles.value = await api.listImportProfiles()
  } catch {
    profiles.value = []
  }
}

const profilesForType = computed(() => profiles.value.filter((p) => !selectedType.value || p.import_type === selectedType.value))

function applyProfile(id: string) {
  const profile = profiles.value.find((p) => p.id === id)
  if (!profile || !previewResult.value) return
  selectedType.value = profile.import_type
  mapping.value = profileMapping(previewResult.value.headers, profile.columns)
  dateOrder.value = profile.date_order
  decimalSeparator.value = profile.decimal_separator
  profileName.value = profile.name
  void handlePreview()
}

async function saveProfile() {
  const name = profileName.value.trim()
  if (!name || !previewResult.value || previewResult.value.type === 'UNKNOWN') return
  error.value = ''
  try {
    await api.saveImportProfile({
      name,
      import_type: previewResult.value.type,
      columns: profileColumns(previewResult.value.mapping),
      date_order: dateOrder.value,
      decimal_separator: decimalSeparator.value,
    })
    await loadProfiles()
  } catch (err: any) {
    error.value = err.message || t('import.profileSaveFailed')
  }
}

async function deleteProfile(id: string) {
  try {
    await api.deleteImportProfile(id)
    await loadProfiles()
  } catch (err: any) {
    error.value = err.message || t('import.profileDeleteFailed')
  }
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
    <div v-dialog="close" class="w-full max-w-2xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
      <!-- Header -->
      <div class="px-6 py-4 border-b border-slate-800 flex items-center justify-between">
        <h3 class="text-lg font-bold text-white flex items-center gap-2">
          <UploadCloud class="w-5 h-5 text-indigo-400" />
          {{ $t('import.modalTitle') }}
        </h3>
        <button @click="close" class="tap p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-6 space-y-5 overflow-y-auto">
        <div v-if="error" class="p-3 bg-danger-500/10 border border-danger-500/20 rounded-xl text-xs text-danger-400">
          {{ error }}
        </div>

        <!-- Result -->
        <div
          v-if="executeResult"
          class="p-4 rounded-2xl space-y-3 border"
          :class="executeResult.committed ? 'bg-success-500/10 border-success-500/30' : 'bg-rose-500/10 border-rose-500/30'"
        >
          <div class="flex items-center gap-2.5 font-bold" :class="executeResult.committed ? 'text-success-400' : 'text-rose-400'">
            <CheckCircle2 v-if="executeResult.committed" class="w-5 h-5" />
            <XCircle v-else class="w-5 h-5" />
            <span>{{ executeResult.committed ? $t('import.successTitle') : $t('import.cancelledTitle') }}</span>
          </div>
          <p v-if="!executeResult.committed" class="text-xs text-slate-300">{{ $t('import.cancelledHint') }}</p>
          <div class="grid grid-cols-3 gap-2 text-center pt-2">
            <div class="p-3 bg-slate-800/80 rounded-xl border border-slate-700/50">
              <span class="text-xs text-slate-400 block">{{ $t('import.imported') }}</span>
              <span class="text-xl font-bold text-success-400">{{ executeResult.imported_count }}</span>
            </div>
            <div class="p-3 bg-slate-800/80 rounded-xl border border-slate-700/50">
              <span class="text-xs text-slate-400 block">{{ $t('import.skipped') }}</span>
              <span class="text-xl font-bold text-warning-400">{{ executeResult.skipped_count }}</span>
            </div>
            <div class="p-3 bg-slate-800/80 rounded-xl border border-slate-700/50">
              <span class="text-xs text-slate-400 block">{{ $t('import.errors') }}</span>
              <span class="text-xl font-bold text-danger-400">{{ executeResult.error_count }}</span>
            </div>
          </div>
          <div v-if="executeResult.errors?.length" class="text-xs text-danger-400 space-y-1 max-h-32 overflow-y-auto pt-2">
            <div v-for="(msg, idx) in rowErrors(executeResult.errors)" :key="idx">• {{ msg }}</div>
            <div v-if="executeResult.errors_truncated" class="text-slate-400">{{ $t('import.moreErrors') }}</div>
          </div>
          <div class="pt-2 flex justify-end">
            <button
              @click="close"
              class="px-4 py-2 bg-success-600 hover:bg-success-500 text-white text-xs font-semibold rounded-xl transition-colors"
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
                <code class="text-success-300">{{ CSV_COLUMNS[type].required.join(', ') }}</code>
                <span v-if="CSV_COLUMNS[type].optional.length"> + <code>{{ CSV_COLUMNS[type].optional.join(', ') }}</code></span>
              </p>
            </div>
          </details>

          <details v-if="batches.length" class="rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2 text-xs text-slate-300">
            <summary class="cursor-pointer font-semibold text-slate-200">{{ $t('import.historyTitle') }}</summary>
            <ul class="mt-2 space-y-2">
              <li v-for="batch in batches" :key="batch.id" class="flex items-center justify-between gap-2">
                <span class="min-w-0 break-words">
                  <span class="font-semibold text-white">{{ $t(`import.type${batch.import_type.charAt(0)}${batch.import_type.slice(1).toLowerCase()}`) }}</span>
                  · {{ new Date(batch.created_at).toLocaleString(intlLocale(), { dateStyle: 'medium', timeStyle: 'short' }) }}
                  · {{ $t('import.historyRows', { remaining: batch.remaining, total: batch.row_count }) }}
                </span>
                <button type="button" @click="undoBatch(batch)" class="shrink-0 text-rose-400 hover:text-rose-300 font-semibold">{{ $t('import.undo') }}</button>
              </li>
            </ul>
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
                class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl shrink-0 transition-colors disabled:opacity-50"
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
                @change="onOptionsChange"
                class="field"
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
                  @change="onOptionsChange"
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
              <span class="text-success-400">{{ previewResult.valid_rows }} {{ $t('import.validRows') }}</span>
              <span class="text-warning-400">{{ previewResult.duplicate_rows }} {{ $t('import.duplicateRows') }}</span>
              <span class="text-danger-400">{{ previewResult.invalid_rows }} {{ $t('import.invalidRows') }}</span>
            </div>
            <p class="text-xs text-slate-400">{{ $t('import.distanceHint', { unit: distanceUnit() }) }}</p>

            <div v-if="previewResult.errors?.length" class="p-3 bg-danger-500/10 border border-danger-500/20 rounded-xl text-xs text-danger-400 space-y-1 max-h-32 overflow-y-auto">
              <div v-for="(msg, idx) in rowErrors(previewResult.errors)" :key="idx">• {{ msg }}</div>
              <div v-if="previewResult.errors_truncated" class="text-slate-400">{{ $t('import.moreErrors') }}</div>
            </div>

            <!-- Column mapping -->
            <details class="rounded-xl border border-slate-800 bg-slate-950/40 px-3 py-2 text-xs text-slate-300">
              <summary class="cursor-pointer font-semibold text-slate-200">{{ $t('import.mappingTitle') }}</summary>
              <p class="mt-2 text-slate-400">{{ $t('import.mappingHint') }}</p>
              <div v-if="profilesForType.length" class="mt-3">
                <label for="csv-profile" class="block text-slate-400 mb-1">{{ $t('import.profileApply') }}</label>
                <div class="flex items-center gap-2">
                  <select id="csv-profile" class="field flex-1" @change="applyProfile(($event.target as HTMLSelectElement).value)">
                    <option value="">{{ $t('import.profileNone') }}</option>
                    <option v-for="p in profilesForType" :key="p.id" :value="p.id">{{ p.name }}</option>
                  </select>
                </div>
              </div>
              <div class="mt-3 space-y-2">
                <div v-for="col in previewResult.mapping" :key="col.index" class="grid grid-cols-2 gap-2 items-center">
                  <label :for="`csv-col-${col.index}`" class="truncate font-mono text-xs text-slate-400" :title="col.header">{{ col.header }}</label>
                  <select
                    :id="`csv-col-${col.index}`"
                    :value="col.field"
                    @change="setColumnField(col.index, ($event.target as HTMLSelectElement).value)"
                    class="field"
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
                  <select id="csv-date-order" v-model="dateOrder" @change="handlePreview" class="field">
                    <option value="">{{ $t('import.autoDetect') }}</option>
                    <option value="dmy">{{ $t('import.dateDmy') }}</option>
                    <option value="mdy">{{ $t('import.dateMdy') }}</option>
                    <option value="ymd">{{ $t('import.dateYmd') }}</option>
                  </select>
                </div>
                <div>
                  <label for="csv-decimal" class="block text-slate-400 mb-1">{{ $t('import.decimalSeparator') }}</label>
                  <select id="csv-decimal" v-model="decimalSeparator" @change="handlePreview" class="field">
                    <option value="">{{ $t('import.autoDetect') }}</option>
                    <option value=".">{{ $t('import.decimalDot') }}</option>
                    <option value=",">{{ $t('import.decimalComma') }}</option>
                  </select>
                </div>
              </div>
              <button type="button" @click="resetFormat" class="mt-3 text-rose-400 hover:text-rose-300 font-semibold">{{ $t('import.resetMapping') }}</button>
              <div v-if="previewResult.type !== 'UNKNOWN'" class="mt-3 border-t border-slate-800 pt-3">
                <label for="csv-profile-name" class="block text-slate-400 mb-1">{{ $t('import.profileSaveLabel') }}</label>
                <div class="flex items-center gap-2">
                  <input id="csv-profile-name" v-model="profileName" maxlength="60" type="text" :placeholder="$t('import.profileNamePlaceholder')" class="field flex-1" />
                  <button type="button" :disabled="!profileName.trim()" @click="saveProfile" class="shrink-0 px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-white font-semibold rounded-lg disabled:opacity-50">{{ $t('common.save') }}</button>
                </div>
                <ul v-if="profilesForType.length" class="mt-2 space-y-1">
                  <li v-for="p in profilesForType" :key="p.id" class="flex items-center justify-between gap-2 text-slate-400">
                    <span class="truncate">{{ p.name }}</span>
                    <button type="button" @click="deleteProfile(p.id)" class="shrink-0 text-danger-400 hover:text-danger-300">{{ $t('common.delete') }}</button>
                  </li>
                </ul>
              </div>
            </details>

            <!-- Sample rows -->
            <div class="overflow-x-auto border border-slate-800 rounded-xl">
              <table class="w-full text-left text-xs text-slate-300">
                <thead class="bg-slate-800/80 text-slate-400 font-semibold border-b border-slate-700/60">
                  <tr>
                    <th v-for="h in previewResult.headers" :key="h" class="px-3 py-2 whitespace-nowrap">{{ h }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-800/60">
                  <tr v-for="(row, idx) in previewResult.sample_rows" :key="idx" class="hover:bg-slate-800/40">
                    <td v-for="h in previewResult.headers" :key="h" class="px-3 py-2 whitespace-nowrap font-mono text-xs">
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
                class="px-5 py-2 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white text-xs font-semibold rounded-xl shadow-lg shadow-rose-600/25 transition-all flex items-center gap-1.5 disabled:opacity-50"
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
