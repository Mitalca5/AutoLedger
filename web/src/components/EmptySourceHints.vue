<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Plus, UploadCloud, Webhook } from 'lucide-vue-next'
import { useQuickAddStore } from '@/stores/quickAdd'
import { useVehicleStore } from '@/stores/vehicle'
import type { QuickKind } from '@/utils/quickAdd'

// Ways to fill an empty list; quickKind is omitted when the quick entry sheet has no tab for the data.
const props = defineProps<{ quickKind?: QuickKind }>()
defineEmits<{ 'import-csv': [] }>()
const router = useRouter()
const quickAdd = useQuickAddStore()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div v-if="vehicleStore.canEdit" class="space-y-2">
    <p class="text-xs text-slate-400">{{ $t('common.emptySources.intro') }}</p>
    <div class="flex flex-wrap justify-center gap-2">
      <button v-if="props.quickKind" type="button" class="empty-hint-btn" @click="quickAdd.open(props.quickKind)">
        <Plus class="w-3.5 h-3.5 text-rose-400" />{{ $t('common.emptySources.quickAdd') }}
      </button>
      <button type="button" class="empty-hint-btn" @click="$emit('import-csv')">
        <UploadCloud class="w-3.5 h-3.5 text-info-400" />{{ $t('common.emptySources.csv') }}
      </button>
      <button type="button" class="empty-hint-btn" @click="router.push('/account')">
        <Webhook class="w-3.5 h-3.5 text-success-400" />{{ $t('common.emptySources.webhook') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.empty-hint-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(226 232 240);
  background: rgb(30 41 59);
  border: 1px solid rgb(51 65 85);
  border-radius: 0.75rem;
  transition: background-color 0.15s;
}
.empty-hint-btn:hover {
  background: rgb(51 65 85);
}
</style>
