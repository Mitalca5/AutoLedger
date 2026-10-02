<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { t } from '@/i18n'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { Database, X, RefreshCw, Link2, CheckCircle2, AlertCircle, Pencil } from 'lucide-vue-next'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

// Where a vehicle's data comes from: the TeslaMate connection and what each origin has written so far.
const props = defineProps<{ vehicle: any | null }>()
const emit = defineEmits<{ edit: [vehicle: any] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))

const ORIGINS = ['TESLAMATE', 'WEBHOOK', 'CSV', 'MANUAL'] as const

const loading = ref(false)
const error = ref('')
const summary = ref<{ teslamate_configured: boolean; activity: any[] } | null>(null)
const test = ref<{ loading: boolean; success?: boolean; error?: string } | null>(null)

const isOwner = computed(() => props.vehicle?.role === 'OWNER')
const rows = computed(() =>
  ORIGINS.map((origin) => ({ origin, ...(summary.value?.activity.find((a) => a.origin === origin) ?? {}) }))
    .filter((r: any) => r.origin !== 'TESLAMATE' || summary.value?.teslamate_configured || r.drives || r.charges)
)

watch(open, async (isOpen) => {
  if (!isOpen || !props.vehicle) return
  summary.value = null
  test.value = null
  error.value = ''
  loading.value = true
  try {
    summary.value = await api.getDataSources(props.vehicle.id)
  } catch (err: any) {
    error.value = err.message
  } finally {
    loading.value = false
  }
})

async function testConnection() {
  if (!props.vehicle) return
  test.value = { loading: true }
  try {
    await api.testTeslaMate(props.vehicle.id)
    test.value = { loading: false, success: true }
  } catch (err: any) {
    test.value = { loading: false, success: false, error: err.message }
  }
}

function formatLast(iso?: string) {
  return iso ? new Date(iso).toLocaleDateString(intlLocale()) : t('vehicles.vehicleDataSourcesModal.never')
}
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden">
      <div class="px-5 py-4 border-b border-slate-800 flex items-center justify-between shrink-0">
        <div class="flex min-w-0 items-center gap-3">
          <div class="p-2 bg-cyan-500/10 text-cyan-400 rounded-xl"><Database class="w-5 h-5" /></div>
          <div class="min-w-0">
            <h3 class="text-base font-bold text-white">{{ $t('vehicles.vehicleDataSourcesModal.title') }}</h3>
            <p class="text-xs text-slate-400 truncate">{{ vehicle?.name }}</p>
          </div>
        </div>
        <button @click="open = false" class="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto space-y-4 text-xs">
        <div v-if="loading" class="flex justify-center py-6"><RefreshCw class="w-5 h-5 animate-spin text-slate-500" /></div>
        <p v-else-if="error" class="text-rose-400">{{ error }}</p>
        <template v-else-if="summary">
          <div v-if="vehicle?.powertrain !== 'ICE'" class="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-3">
            <div class="flex items-center justify-between gap-2">
              <h4 class="font-bold text-white uppercase tracking-wider">{{ $t('vehicles.vehicleDataSourcesModal.teslamate') }}</h4>
              <span class="font-semibold" :class="summary.teslamate_configured ? 'text-emerald-400' : 'text-slate-500'">
                {{ !isOwner ? $t('vehicles.vehicleCard.managedByAdmin') : summary.teslamate_configured ? $t('vehicles.vehicleCard.configured') : $t('vehicles.vehicleCard.notConfigured') }}
              </span>
            </div>
            <p class="text-slate-400">{{ $t('vehicles.vehicleDataSourcesModal.teslamateHint') }}</p>
            <div v-if="isOwner" class="flex flex-wrap gap-2">
              <button
                v-if="summary.teslamate_configured"
                @click="testConnection"
                :disabled="test?.loading"
                class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
              >
                <RefreshCw v-if="test?.loading" class="w-3.5 h-3.5 animate-spin" />
                <Link2 v-else class="w-3.5 h-3.5" />
                {{ $t('vehicles.vehicleCard.testApi') }}
              </button>
              <button
                @click="emit('edit', vehicle); open = false"
                class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700"
              >
                <Pencil class="w-3.5 h-3.5" />
                {{ summary.teslamate_configured ? $t('vehicles.vehicleDataSourcesModal.editConnection') : $t('vehicles.vehicleDataSourcesModal.connect') }}
              </button>
            </div>
            <p v-if="test && !test.loading" class="flex items-start gap-2" :class="test.success ? 'text-emerald-300' : 'text-rose-300'">
              <CheckCircle2 v-if="test.success" class="w-4 h-4 shrink-0" />
              <AlertCircle v-else class="w-4 h-4 shrink-0" />
              <span>{{ test.success ? $t('vehicles.vehicleCard.online') : test.error }}</span>
            </p>
          </div>

          <div>
            <h4 class="font-bold text-white uppercase tracking-wider mb-2">{{ $t('vehicles.vehicleDataSourcesModal.activity') }}</h4>
            <ul class="space-y-2">
              <li v-for="r in rows" :key="r.origin" class="bg-slate-950/60 border border-slate-800 rounded-xl p-3">
                <div class="flex items-center justify-between gap-2">
                  <span class="font-semibold text-slate-200">{{ $t('vehicles.vehicleDataSourcesModal.origin.' + r.origin) }}</span>
                  <span class="text-slate-500">{{ $t('vehicles.vehicleDataSourcesModal.lastData', { date: formatLast((r as any).last_at) }) }}</span>
                </div>
                <p class="mt-1 text-slate-400">
                  {{ $t('vehicles.vehicleDataSourcesModal.counts', { drives: (r as any).drives || 0, charges: (r as any).charges || 0, readings: (r as any).odometer_readings || 0 }) }}
                </p>
              </li>
            </ul>
            <p v-if="!summary.activity.length" class="text-slate-500 mt-2">{{ $t('vehicles.vehicleDataSourcesModal.empty') }}</p>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
