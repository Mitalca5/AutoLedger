<script setup lang="ts">
import { AlertTriangle, RefreshCw } from 'lucide-vue-next'

defineProps<{ message?: string }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <div role="alert" class="rounded-2xl border border-danger-500/30 bg-danger-500/10 p-4 flex flex-col sm:flex-row sm:items-center gap-3 text-sm">
    <AlertTriangle class="w-5 h-5 text-danger-400 shrink-0" aria-hidden="true" />
    <div class="flex-1 min-w-0">
      <p class="font-semibold text-danger-200">{{ $t('common.loadFailed') }}</p>
      <p v-if="message" class="text-xs text-danger-300/80 mt-0.5 break-words">{{ message }}</p>
    </div>
    <button type="button" class="btn btn-secondary tap shrink-0" @click="$emit('retry')">
      <RefreshCw class="w-3.5 h-3.5" aria-hidden="true" />
      {{ $t('common.retry') }}
    </button>
  </div>
</template>
