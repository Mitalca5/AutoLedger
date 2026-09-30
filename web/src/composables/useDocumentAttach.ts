import { ref } from 'vue'
import { t } from '@/i18n'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'

interface DocumentFields {
  document_id: string | null
  document_filename: string | null
}

/**
 * Attaching a receipt to an expense form: pick an existing document or upload a new one.
 * A newly uploaded document is reported through onAdded so the page can add it to its list.
 */
export function useDocumentAttach(
  vehicleId: () => string,
  documents: () => ExpenseDocumentHeader[],
  onAdded: (doc: ExpenseDocumentHeader) => void
) {
  const { showAlert } = useConfirm()
  const isUploadingDocument = ref(false)

  function onSelectExistingDoc(docId: string, form: DocumentFields) {
    if (!docId) {
      form.document_id = null
      form.document_filename = null
      return
    }
    const found = documents().find((d) => d.id === docId)
    if (found) {
      form.document_id = found.id
      form.document_filename = found.filename
    }
  }

  async function upload(file: File, form: DocumentFields) {
    isUploadingDocument.value = true
    try {
      const doc = await api.uploadDocument(vehicleId(), file)
      onAdded(doc)
      form.document_id = doc.id
      form.document_filename = doc.filename
      void showAlert(t('shell.documents.attached', { filename: doc.filename }), t('common.success'), 'success')
    } catch (err: any) {
      void showAlert(t('shell.documents.uploadError', { message: err.message }), t('shell.confirm.error'), 'danger')
    } finally {
      isUploadingDocument.value = false
    }
  }

  async function onFileInputChange(event: Event, form: DocumentFields) {
    const input = event.target as HTMLInputElement
    if (!input.files || !input.files.length) return
    if (!vehicleId()) return
    await upload(input.files[0], form)
    input.value = ''
  }

  async function onDropzoneDirectUpload(file: File | null, form: DocumentFields) {
    if (!file || !vehicleId()) return
    await upload(file, form)
  }

  return { isUploadingDocument, onSelectExistingDoc, onFileInputChange, onDropzoneDirectUpload }
}
