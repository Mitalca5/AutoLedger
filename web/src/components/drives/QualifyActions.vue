<script setup lang="ts">
import { Plus, Ban } from 'lucide-vue-next'

// The two taps that qualify an item of a "to qualify" queue: accept it (enter the toll, create the trip)
// or rule it out (no toll, not a trip).
defineProps<{ primaryLabel: string; primaryTitle: string; secondaryLabel: string; secondaryTitle: string; busy?: boolean }>()
const emit = defineEmits<{ primary: []; secondary: [] }>()
</script>

<template>
  <div class="flex items-center gap-1">
    <button
      type="button"
      :disabled="busy"
      @click="emit('primary')"
      class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-warning-500/40 bg-warning-500/10 text-warning-400 hover:bg-warning-500/20 flex items-center gap-1 disabled:opacity-50"
      :title="primaryTitle"
    >
      <Plus class="w-3.5 h-3.5" /> {{ primaryLabel }}
    </button>
    <button
      type="button"
      :disabled="busy"
      @click="emit('secondary')"
      class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-400 hover:text-white flex items-center gap-1 disabled:opacity-50"
      :title="secondaryTitle"
    >
      <Ban class="w-3.5 h-3.5" /> {{ secondaryLabel }}
    </button>
  </div>
</template>
