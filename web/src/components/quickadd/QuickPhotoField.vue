<script setup lang="ts">
import { t } from '@/i18n'
import { ref } from 'vue'
import { Camera, FileText, X } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useOfflineStore } from '@/stores/offline'

const props = defineProps<{
  vehicleId: string
  documentId: string | null
  filename: string | null
}>()

const emit = defineEmits<{
  'update:documentId': [value: string | null]
  'update:filename': [value: string | null]
}>()

const offlineStore = useOfflineStore()
const uploading = ref(false)
const error = ref('')

// The receipt is uploaded as soon as it is taken so the entry only carries its identifier.
async function onFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  uploading.value = true
  error.value = ''
  try {
    const doc = await api.uploadDocument(props.vehicleId, file)
    emit('update:documentId', doc.id)
    emit('update:filename', doc.filename)
  } catch (err: any) {
    error.value = err?.message || t('quickadd.quickPhotoField.uploadFailed')
  } finally {
    uploading.value = false
    input.value = ''
  }
}

function clear() {
  emit('update:documentId', null)
  emit('update:filename', null)
}
</script>

<template>
  <div>
    <div v-if="documentId" class="flex min-h-12 items-center justify-between gap-2 rounded-xl border border-indigo-500/30 bg-slate-800 px-3">
      <span class="flex min-w-0 items-center gap-2 text-sm text-white">
        <FileText class="h-4 w-4 shrink-0 text-indigo-400" aria-hidden="true" />
        <span class="truncate">{{ filename || $t('quickadd.quickPhotoField.attached') }}</span>
      </span>
      <button type="button" class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-slate-400 hover:text-rose-400" :aria-label="$t('quickadd.quickPhotoField.removeTheReceipt')" @click="clear">
        <X class="h-4 w-4" aria-hidden="true" />
      </button>
    </div>

    <template v-else>
      <label
        class="flex min-h-12 items-center justify-center gap-2 rounded-xl border border-indigo-500/30 bg-indigo-600/20 px-3 text-sm font-semibold text-indigo-200"
        :class="offlineStore.isOnline && !uploading ? 'cursor-pointer hover:bg-indigo-600/30' : 'opacity-60'"
      >
        <Camera class="h-4 w-4" aria-hidden="true" />
        {{ uploading ? $t('quickadd.quickPhotoField.uploading') : $t('quickadd.quickPhotoField.photo') }}
        <input type="file" accept="image/*" capture="environment" class="sr-only" :disabled="!offlineStore.isOnline || uploading" @change="onFile" />
      </label>
      <p v-if="!offlineStore.isOnline" class="mt-1 text-xs text-slate-400">{{ $t('quickadd.quickPhotoField.photoUnavailableOfflineAttachIt') }}</p>
    </template>
    <p v-if="error" role="alert" class="mt-1 text-xs text-rose-300">{{ error }}</p>
  </div>
</template>
