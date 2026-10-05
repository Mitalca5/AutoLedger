<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { MapPin } from 'lucide-vue-next'
import { api } from '@/services/api'
import { t } from '@/i18n'

// Tells when drives hold a position instead of an address, and lets an editor resolve them once the server
// has reverse geocoding on.
const props = defineProps<{ vehicleId: string; canEdit: boolean }>()
const emit = defineEmits<{ resolved: [] }>()

const POLL_MS = 3000
const status = ref<{ geocoding_enabled: boolean; pending: number; running: boolean } | null>(null)
const queued = ref(0)
const error = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
let wasRunning = false

async function refresh() {
  clearTimeout(timer)
  if (!props.vehicleId) {
    status.value = null
    return
  }
  try {
    status.value = await api.getAddressBackfill(props.vehicleId)
  } catch {
    status.value = null
    return
  }
  if (status.value.running) {
    wasRunning = true
    timer = setTimeout(refresh, POLL_MS)
  } else if (wasRunning) {
    wasRunning = false
    queued.value = 0
    emit('resolved')
  }
}

async function resolve() {
  error.value = ''
  try {
    const res = await api.resolveDriveAddresses(props.vehicleId)
    queued.value = res.queued
    wasRunning = res.queued > 0
  } catch (err: any) {
    error.value = t('drives.addressBackfill.error', { message: err.message })
    return
  }
  await refresh()
}

watch(() => props.vehicleId, refresh, { immediate: true })
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div
    v-if="status && (status.pending > 0 || status.running)"
    class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-info-500/30 bg-info-500/10 p-4 text-sm text-info-200"
  >
    <span class="flex items-start gap-2 min-w-0 flex-1 basis-64">
      <MapPin class="w-4 h-4 mt-0.5 shrink-0" />
      <span>
        <template v-if="!status.geocoding_enabled">{{ $t('drives.addressBackfill.info', { count: status.pending }) }}</template>
        <template v-else-if="status.running">{{ $t('drives.addressBackfill.queued', { count: queued || status.pending }) }}</template>
        <template v-else>{{ $t('drives.addressBackfill.ready', { count: status.pending }) }}</template>
        <span v-if="error" class="block text-danger-300 mt-1">{{ error }}</span>
      </span>
    </span>
    <button
      v-if="status.geocoding_enabled && canEdit"
      type="button"
      :disabled="status.running"
      @click="resolve"
      class="rounded-lg bg-info-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-info-500 transition-colors disabled:opacity-60"
    >
      {{ status.running ? $t('drives.addressBackfill.running') : $t('drives.addressBackfill.resolve') }}
    </button>
  </div>
</template>
