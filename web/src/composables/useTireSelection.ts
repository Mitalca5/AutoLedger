import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { MOUNTED_POSITIONS } from '@/utils/tires'

interface TireSelectionSource {
  tires: Ref<any[]>
  mountedTires: ComputedRef<Record<string, any>>
  storageTires: ComputedRef<any[]>
  disposedTires: ComputedRef<any[]>
  activeTab: Ref<'chassis' | 'storage' | 'disposed'>
}

// Selection of tires for the batch actions, and the select-all toggle of the tab on display.
export function useTireSelection({ tires, mountedTires, storageTires, disposedTires, activeTab }: TireSelectionSource) {
  // Selection for batch actions
  const selectedTireIds = ref<string[]>([])
  function toggleTireSelection(id: string) {
    selectedTireIds.value = selectedTireIds.value.includes(id)
      ? selectedTireIds.value.filter((x) => x !== id)
      : [...selectedTireIds.value, id]
  }

  const currentTabTireIds = computed<string[]>(() => {
    if (activeTab.value === 'chassis') {
      return MOUNTED_POSITIONS.map((pos) => mountedTires.value[pos]?.tire.id).filter(Boolean)
    }
    if (activeTab.value === 'storage') {
      return storageTires.value.map((t) => t.tire.id)
    }
    if (activeTab.value === 'disposed') {
      return disposedTires.value.map((t) => t.tire.id)
    }
    return []
  })

  const isCurrentTabAllSelected = computed<boolean>(() => {
    const ids = currentTabTireIds.value
    return ids.length > 0 && ids.every((id) => selectedTireIds.value.includes(id))
  })

  const isCurrentTabPartlySelected = computed<boolean>(
    () => !isCurrentTabAllSelected.value && currentTabTireIds.value.some((id) => selectedTireIds.value.includes(id))
  )

  function toggleSelectAllCurrentTab() {
    const ids = currentTabTireIds.value
    if (!ids.length) return
    if (isCurrentTabAllSelected.value) {
      selectedTireIds.value = selectedTireIds.value.filter((id) => !ids.includes(id))
    } else {
      selectedTireIds.value = Array.from(new Set([...selectedTireIds.value, ...ids]))
    }
  }

  const selectedDisposedCount = computed(() => {
    return tires.value.filter((t) => selectedTireIds.value.includes(t.tire.id) && t.tire.current_position === 'DISPOSED').length
  })
  const canBatchDispose = computed(() => {
    return selectedTireIds.value.length > 0 && selectedTireIds.value.length > selectedDisposedCount.value
  })

  return {
    selectedTireIds,
    toggleTireSelection,
    currentTabTireIds,
    isCurrentTabAllSelected,
    isCurrentTabPartlySelected,
    toggleSelectAllCurrentTab,
    canBatchDispose,
  }
}
