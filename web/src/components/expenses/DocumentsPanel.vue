<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import { useVehicleStore } from '@/stores/vehicle'
import type { ExpenseDocumentHeader } from '@/services/api'
import { Trash2, Paperclip, FileText, Download, Eye, UploadCloud, Loader2 } from 'lucide-vue-next'
import { formatDate, formatFileSize } from '@/utils/expenses'

defineProps<{ documents: ExpenseDocumentHeader[]; loading: boolean; loadingDocId: string | null }>()
const emit = defineEmits<{
  upload: []
  delete: [document: ExpenseDocumentHeader]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div class="space-y-4">
    <!-- Info banner -->
    <div class="p-4 bg-indigo-500/10 border border-indigo-500/20 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs text-indigo-300">
      <div class="flex items-center gap-2.5">
        <Paperclip class="w-4 h-4 text-indigo-400 shrink-0" />
        <span>
          {{ $t('expenses.documentsPanel.receiptsInvoicesTicketsWorkshopReports') }}
        </span>
      </div>
      <span class="font-semibold shrink-0">
        {{ $t('expenses.documentsPanel.documentS', { length: documents.length }) }}
      </span>
    </div>

    <div v-if="loading" class="text-center py-12 text-slate-400">{{ $t('common.loading') }}</div>
    <EmptyState v-else-if="!documents.length">
      {{ $t('expenses.documentsPanel.noReceiptOrInvoiceUploaded') }}
      <template #actions>
      <button
        v-if="vehicleStore.canEdit"
        @click="emit('upload')"
        class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl inline-flex items-center gap-2 shadow-lg shadow-rose-600/20"
      >
        <UploadCloud class="w-4 h-4" />
        {{ $t('expenses.documentsPanel.uploadAFirstDocument') }}
      </button>
      </template>
    </EmptyState>

    <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-3">
      <div
        v-for="d in documents"
        :key="d.id"
        class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col justify-between gap-3 hover:border-slate-700 transition-colors"
      >
        <div class="space-y-2">
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-center gap-2.5 min-w-0">
              <div class="p-2 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 shrink-0">
                <FileText class="w-5 h-5" />
              </div>
              <div class="min-w-0">
                <h4 class="text-sm font-semibold text-white truncate" :title="d.filename">
                  {{ d.filename }}
                </h4>
                <p class="text-xs text-slate-400">
                  {{ formatDate(d.created_at) }} • {{ formatFileSize(d.file_size) }}
                </p>
              </div>
            </div>
            <span
              class="text-xs px-2 py-0.5 rounded-full font-bold shrink-0 border"
              :class="d.linked_expenses_count > 0 ? 'bg-success-500/10 text-success-400 border-success-500/20' : 'bg-slate-800 text-slate-400 border-slate-700'"
            >
              {{ d.linked_expenses_count > 0 ? $t('expenses.documentsPanel.linkedExpenses', { count: d.linked_expenses_count }) : $t('expenses.documentsPanel.notLinked') }}
            </span>
          </div>

          <p v-if="d.description" class="text-xs text-slate-300 italic pl-1">
            « {{ d.description }} »
          </p>
        </div>

        <div class="flex items-center justify-between pt-2 border-t border-slate-800/80">
          <div class="flex items-center gap-1.5">
            <button
              @click="emit('view-document', d.id, d.filename, false)"
              :disabled="loadingDocId === d.id"
              class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 border border-slate-700/60 transition-colors disabled:opacity-50"
              :title="$t('expenses.documentsPanel.viewTheFile')"
            >
              <Loader2 v-if="loadingDocId === d.id" class="w-3.5 h-3.5 text-indigo-400 animate-spin" />
              <Eye v-else class="w-3.5 h-3.5 text-indigo-400" />
              <span>{{ $t('expenses.documentsPanel.open') }}</span>
            </button>
            <button
              @click="emit('view-document', d.id, d.filename, true)"
              class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 border border-slate-700/60 transition-colors"
              :title="$t('expenses.documentsPanel.downloadTheFile')"
            >
              <Download class="w-3.5 h-3.5 text-indigo-400" />
              <span>{{ $t('expenses.documentsPanel.download') }}</span>
            </button>
          </div>

          <button
            v-if="vehicleStore.canEdit"
            @click="emit('delete', d)"
            class="tap p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
            :title="$t('expenses.documentsPanel.deleteThisDocument')"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
