<script setup lang="ts">
import { FileText, Image as ImageIcon, Paperclip, Download, ExternalLink, X } from 'lucide-vue-next'

export interface DocumentPreviewState {
  id: string
  filename: string
  url: string
  isPdf: boolean
  isImage: boolean
}

const props = defineProps<{
  previewDoc: DocumentPreviewState | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

function downloadFile() {
  if (!props.previewDoc) return
  const a = document.createElement('a')
  a.href = props.previewDoc.url
  a.download = props.previewDoc.filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function openInNewTab() {
  if (!props.previewDoc) return
  window.open(props.previewDoc.url, '_blank')
}
</script>

<template>
  <div
    v-if="previewDoc"
    class="fixed inset-0 z-modal-nested bg-black/80 backdrop-blur-sm flex items-center justify-center p-2 sm:p-4 overflow-hidden"
    @click.self="emit('close')"
  >
    <div v-dialog="() => emit('close')" class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-5xl h-[92vh] sm:h-[90vh] flex flex-col shadow-2xl overflow-hidden">
      <!-- Header -->
      <div class="px-4 sm:px-6 py-3.5 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95 gap-3">
        <div class="flex items-center gap-2.5 min-w-0">
          <div class="w-8 h-8 rounded-xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-center shrink-0">
            <FileText v-if="previewDoc.isPdf" class="w-4 h-4 text-sky-400" />
            <ImageIcon v-else-if="previewDoc.isImage" class="w-4 h-4 text-success-400" />
            <Paperclip v-else class="w-4 h-4 text-slate-400" />
          </div>
          <div class="min-w-0">
            <h3 class="text-sm font-bold text-white truncate" :title="previewDoc.filename">
              {{ previewDoc.filename }}
            </h3>
            <p class="text-xs text-slate-400 flex items-center gap-1.5">
              <span v-if="previewDoc.isPdf" class="text-sky-400 font-semibold">{{ $t('expenses.documentPreviewModal.pdfDocument') }}</span>
              <span v-else-if="previewDoc.isImage" class="text-success-400 font-semibold">{{ $t('expenses.documentPreviewModal.image') }}</span>
              <span v-else class="text-slate-400 font-semibold">{{ $t('expenses.documentPreviewModal.file') }}</span>
            </p>
          </div>
        </div>

        <div class="flex items-center gap-1.5 shrink-0">
          <!-- Download Button -->
          <button
            type="button"
            @click="downloadFile"
            class="btn btn-secondary"
            :title="$t('expenses.documentPreviewModal.downloadTheFile')"
          >
            <Download class="w-3.5 h-3.5 text-sky-400" />
            <span class="hidden sm:inline">{{ $t('expenses.documentPreviewModal.download') }}</span>
          </button>

          <!-- Open in New Tab Button -->
          <button
            type="button"
            @click="openInNewTab"
            class="btn btn-secondary"
            :title="$t('expenses.documentPreviewModal.openInANewTab')"
          >
            <ExternalLink class="w-3.5 h-3.5 text-slate-400" />
            <span class="hidden md:inline">{{ $t('expenses.documentPreviewModal.newTab') }}</span>
          </button>

          <!-- Close Button -->
          <button
            type="button"
            @click="emit('close')"
            class="tap text-slate-400 hover:text-white p-1.5 rounded-xl hover:bg-slate-800 transition-colors ml-1"
            :title="$t('common.close')" :aria-label="$t('common.close')"
          >
            <X class="w-5 h-5" />
          </button>
        </div>
      </div>

      <!-- Preview Body -->
      <div class="flex-1 bg-slate-950/70 overflow-hidden flex items-center justify-center min-h-0 relative">
        <!-- PDF preview -->
        <iframe
          v-if="previewDoc.isPdf"
          :src="previewDoc.url"
          class="w-full h-full border-0 bg-white"
          :title="previewDoc.filename"
        />

        <!-- Image preview -->
        <div
          v-else-if="previewDoc.isImage"
          class="w-full h-full p-4 flex items-center justify-center overflow-auto"
        >
          <img
            :src="previewDoc.url"
            :alt="previewDoc.filename"
            class="max-w-full max-h-full object-contain rounded-lg shadow-lg"
          />
        </div>

        <!-- Unsupported preview fallback -->
        <div v-else class="p-8 text-center space-y-3">
          <FileText class="w-12 h-12 text-slate-400 mx-auto" />
          <p class="text-sm text-slate-300">{{ $t('expenses.documentPreviewModal.thisFileFormatCannotBe') }}</p>
          <button
            type="button"
            @click="downloadFile"
            class="btn btn-lg btn-primary"
          >
            <Download class="w-4 h-4" />
            {{ $t('expenses.documentPreviewModal.downloadToView') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
