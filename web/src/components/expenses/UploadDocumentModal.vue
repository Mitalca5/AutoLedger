<script setup lang="ts">
import { t } from '@/i18n'
import { ref, watch } from 'vue'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { X, UploadCloud } from 'lucide-vue-next'
import AppDropzone from '@/components/AppDropzone.vue'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

// Adds a receipt that is not attached to any expense yet
const props = defineProps<{ vehicleId: string }>()
const emit = defineEmits<{ 'document-added': [doc: ExpenseDocumentHeader] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const isUploadingDocument = ref(false)
const uploadDocDescription = ref('')
const uploadDocFile = ref<File | null>(null)

watch(open, (isOpen) => {
  if (!isOpen) return
  uploadDocDescription.value = ''
  uploadDocFile.value = null
})

async function handleUploadStandaloneDocument() {
  if (!props.vehicleId || !uploadDocFile.value) {
    showAlert(t('expenses.uploadDocumentModal.selectFile'), t('common.requiredField'), 'warning')
    return
  }
  isUploadingDocument.value = true
  try {
    const doc = await api.uploadDocument(props.vehicleId, uploadDocFile.value, uploadDocDescription.value)
    emit('document-added', doc)
    open.value = false
    showAlert(t('expenses.uploadDocumentModal.added', { filename: doc.filename }), t('common.success'), 'success')
  } catch (err: any) {
    showAlert(t('shell.documents.uploadError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    isUploadingDocument.value = false
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <UploadCloud class="w-5 h-5 text-indigo-400" />
          {{ $t('expenses.uploadDocumentModal.addAReceiptOrAn') }}
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="standalone-doc-form" @submit.prevent="handleUploadStandaloneDocument" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <div>
          <span class="block text-xs font-semibold text-slate-300 mb-1.5">{{ $t('expenses.uploadDocumentModal.receiptFile') }}</span>
          <AppDropzone
            v-model="uploadDocFile"
            :disabled="isUploadingDocument"
            :label="$t('expenses.uploadDocumentModal.dragYourInvoiceHereOr')"
            
          />
        </div>

        <div>
          <label for="standalone-doc-desc" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.uploadDocumentModal.descriptionInvoiceRefOptional') }}</label>
          <input
            id="standalone-doc-desc"
            v-model="uploadDocDescription"
            :placeholder="$t('expenses.uploadDocumentModal.eGServiceInvoiceAugust')"
            class="field"
          />
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
          {{ $t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="standalone-doc-form"
          :disabled="isUploadingDocument"
          class="btn btn-lg btn-primary"
        >
          <UploadCloud class="w-4 h-4" />
          <span>{{ isUploadingDocument ? $t('expenses.uploading') : $t('expenses.uploadDocumentModal.upload') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
