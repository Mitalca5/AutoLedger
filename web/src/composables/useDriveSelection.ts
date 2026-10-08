import { computed, ref, type Ref } from 'vue'

interface SelectableDrive {
  id: string
  start_time: string
  [key: string]: any
}

// Multi-selection of drives for trip grouping, tolls and carpooling. Drives are kept by id so that the
// selection survives pagination and filters.
export function useDriveSelection<T extends SelectableDrive>(drives: Ref<T[]>) {
  const selectedDrives = ref<Record<string, T>>({})
  const selectedDriveIds = computed(() => Object.keys(selectedDrives.value))
  const selectedList = computed(() =>
    Object.values(selectedDrives.value).sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
  )
  const selectedOffPage = computed(() => selectedDriveIds.value.filter((id) => !drives.value.some((d) => d.id === id)).length)
  const allPageSelected = computed(() => drives.value.length > 0 && drives.value.every((d) => selectedDrives.value[d.id]))
  const somePageSelected = computed(() => !allPageSelected.value && drives.value.some((d) => selectedDrives.value[d.id]))

  function clearSelection() {
    selectedDrives.value = {}
  }

  function toggleSelectDrive(d: T) {
    const next = { ...selectedDrives.value }
    if (next[d.id]) {
      delete next[d.id]
    } else {
      next[d.id] = d
    }
    selectedDrives.value = next
  }

  // Selects or unselects the drives of the current page, keeping selections made on other pages
  function selectAll() {
    const next = { ...selectedDrives.value }
    const select = !allPageSelected.value
    for (const d of drives.value) {
      if (select) next[d.id] = d
      else delete next[d.id]
    }
    selectedDrives.value = next
  }

  return { selectedDrives, selectedDriveIds, selectedList, selectedOffPage, allPageSelected, somePageSelected, clearSelection, toggleSelectDrive, selectAll }
}
