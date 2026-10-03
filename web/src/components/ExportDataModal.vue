<script setup lang="ts">
import { ref, watch } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { X, Download } from 'lucide-vue-next'

const props = defineProps<{ open: boolean; vehicleId: string }>()
const emit = defineEmits<{ (e: 'update:open', val: boolean): void }>()

const TYPES = ['charges', 'drives', 'fuel', 'odometer', 'expenses', 'maintenance'] as const

const type = ref<(typeof TYPES)[number]>('drives')
const format = ref<'csv' | 'json'>('csv')
const from = ref('')
const to = ref('')
const loading = ref(false)
const error = ref('')

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) error.value = ''
  },
)

async function download() {
  loading.value = true
  error.value = ''
  try {
    const { blob, filename } = await api.downloadExport(props.vehicleId, {
      type: type.value,
      format: format.value,
      from: from.value,
      to: to.value,
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    emit('update:open', false)
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('import.exportFailed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[9999] flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm overflow-y-auto"
      @click.self="emit('update:open', false)"
    >
      <div class="relative w-full max-w-md bg-slate-900 border border-slate-800 rounded-3xl p-6 shadow-2xl space-y-4 my-auto" role="dialog" aria-modal="true">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-bold text-white">{{ $t('import.exportTitle') }}</h2>
          <button type="button" class="text-slate-400 hover:text-white" :aria-label="$t('common.close')" @click="emit('update:open', false)">
            <X class="w-5 h-5" />
          </button>
        </div>
        <p class="text-xs text-slate-400">{{ $t('import.exportIntro') }}</p>

        <label class="block space-y-1">
          <span class="text-xs font-semibold text-slate-300">{{ $t('import.exportType') }}</span>
          <select v-model="type" class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white">
            <option v-for="k in TYPES" :key="k" :value="k">{{ $t(`import.exportTypes.${k}`) }}</option>
          </select>
        </label>
        <label class="block space-y-1">
          <span class="text-xs font-semibold text-slate-300">{{ $t('import.exportFormat') }}</span>
          <select v-model="format" class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white">
            <option value="csv">CSV</option>
            <option value="json">JSON</option>
          </select>
        </label>
        <div class="grid grid-cols-2 gap-3">
          <label class="block space-y-1">
            <span class="text-xs font-semibold text-slate-300">{{ $t('import.exportFrom') }}</span>
            <input v-model="from" type="date" class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" />
          </label>
          <label class="block space-y-1">
            <span class="text-xs font-semibold text-slate-300">{{ $t('import.exportTo') }}</span>
            <input v-model="to" type="date" class="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" />
          </label>
        </div>

        <p v-if="error" class="text-xs text-rose-400" role="alert">{{ error }}</p>
        <button
          type="button"
          class="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-cyan-500/20 text-cyan-200 border border-cyan-500/30 text-sm font-semibold disabled:opacity-50"
          :disabled="loading"
          @click="download"
        >
          <Download class="w-4 h-4" />
          {{ $t('import.exportDownload') }}
        </button>
      </div>
    </div>
  </Teleport>
</template>
