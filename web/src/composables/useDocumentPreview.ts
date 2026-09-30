import { onUnmounted, ref } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

export interface DocumentPreviewState {
  id: string
  url: string
  filename: string
  isPdf: boolean
  isImage: boolean
}

/** Opens an attached document in the in-app preview, or downloads it. vehicleId is read when the action runs. */
export function useDocumentPreview(vehicleId: () => string | undefined) {
  const { showAlert } = useConfirm()
  const previewDoc = ref<DocumentPreviewState | null>(null)
  const loadingDocId = ref<string | null>(null)

  function closeDocPreview() {
    if (previewDoc.value?.url) {
      URL.revokeObjectURL(previewDoc.value.url)
    }
    previewDoc.value = null
  }

  useEscapeToClose(() => !!previewDoc.value, closeDocPreview)

  onUnmounted(closeDocPreview)

  async function viewOrDownloadDocument(docId: string | null | undefined, filename?: string | null, download = false) {
    const id = vehicleId()
    if (!id || !docId) return
    loadingDocId.value = docId
    try {
      const { blob, filename: serverFilename } = await api.downloadDocumentBlob(id, docId)
      const finalName = filename || serverFilename || 'document'
      const blobUrl = URL.createObjectURL(blob)
      if (download) {
        const a = document.createElement('a')
        a.href = blobUrl
        a.download = finalName
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        setTimeout(() => URL.revokeObjectURL(blobUrl), 1000)
      } else {
        closeDocPreview()
        const lower = finalName.toLowerCase()
        const mime = (blob.type || '').toLowerCase()
        const isPdf = lower.endsWith('.pdf') || mime.includes('pdf')
        const isImage = /\.(png|jpe?g|webp|gif|svg)$/i.test(lower) || mime.startsWith('image/')
        previewDoc.value = {
          id: docId,
          url: blobUrl,
          filename: finalName,
          isPdf,
          isImage,
        }
      }
    } catch (err: any) {
      void showAlert(t('shell.documents.accessError', { message: err.message }), t('shell.confirm.error'), 'danger')
    } finally {
      loadingDocId.value = null
    }
  }

  return { previewDoc, loadingDocId, closeDocPreview, viewOrDownloadDocument }
}
