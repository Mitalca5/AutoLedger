<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import { t } from '@/i18n'
import { ref, watch } from 'vue'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { UploadCloud } from 'lucide-vue-next'
import AppDropzone from '@/components/AppDropzone.vue'

// Adds a receipt that is not attached to any expense yet
const props = defineProps<{ vehicleId: string }>()
const emit = defineEmits<{ 'document-added': [doc: ExpenseDocumentHeader] }>()
const open = defineModel<boolean>('open', { required: true })
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
  <ModalShell
    v-model:open="open"
    :title="$t('expenses.uploadDocumentModal.addAReceiptOrAn')"
    :icon="UploadCloud"
    icon-class="text-indigo-400"
    size="sm"
  >
    <form id="standalone-doc-form" @submit.prevent="handleUploadStandaloneDocument" class="space-y-4">
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
    <template #footer>
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
    </template>
  </ModalShell>
</template>