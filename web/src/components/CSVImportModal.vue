<script setup lang="ts">
import { ref, watch } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import type { CSVExecuteResult, CSVImportType, CSVPreviewResult } from '@/services/csvImport'
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
const loading = ref(false)
const error = ref('')

const previewResult = ref<CSVPreviewResult | null>(null)
const executeResult = ref<CSVExecuteResult | null>(null)

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) return
    file.value = null
    selectedType.value = props.defaultType || ''
    skipDuplicates.value = true
    loading.value = false
    error.value = ''
    previewResult.value = null
    executeResult.value = null
  },
  { immediate: true },
)

watch([selectedType, skipDuplicates], () => {
  previewResult.value = null
})

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
    const res = await api.previewCSVImport(targetVehicleId, file.value, selectedType.value || undefined, skipDuplicates.value)
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
    const res = await api.executeCSVImport(
      targetVehicleId,
      file.value,
      selectedType.value || undefined,
      skipDuplicates.value,
    )
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
