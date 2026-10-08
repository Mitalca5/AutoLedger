import { computed, ref } from 'vue'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'

// The tires of the active vehicle, loaded from the API, and the groups the page shows them in.
export function useTireList() {
  const vehicleStore = useVehicleStore()
  const tires = ref<any[]>([])
  const loading = ref(false)
  const loadError = ref('')
  const loadFailed = ref(false)
  const loadedVehicleId = ref('')

  const vehicleId = computed(() => vehicleStore.activeVehicle?.id ?? '')
  const ready = computed(() => loadedVehicleId.value === vehicleId.value)

  // Mounted tires mapped by position
  const mountedTires = computed(() => {
    const map: Record<string, any> = { FL: null, FR: null, RL: null, RR: null }
    tires.value.forEach((t) => {
      const pos = t.tire.current_position
      if (pos in map) {
        map[pos] = t
      }
    })
    return map
  })

  const hasMountedTires = computed(() => Object.values(mountedTires.value).some(Boolean))

  const storageTires = computed(() => {
    return tires.value.filter((t) => t.tire.current_position === 'STORAGE')
  })

  const disposedTires = computed(() => tires.value.filter((t) => t.tire.current_position === 'DISPOSED'))

  async function loadTires() {
    if (!vehicleStore.activeVehicle) return
    const id = vehicleStore.activeVehicle.id
    loading.value = true
    loadError.value = ''
    loadFailed.value = false
    try {
      const res = (await api.getTires(id)) ?? []
      if (vehicleStore.activeVehicle?.id !== id) return
      tires.value = res
      loadedVehicleId.value = id
    } catch (err: any) {
      console.error('Failed to load tires', err)
      loadError.value = err?.message || ''
      loadFailed.value = true
      if (loadedVehicleId.value !== id) tires.value = []
    } finally {
      loading.value = false
    }
  }

  return {
    tires,
    loading,
    loadError,
    loadFailed,
    ready,
    vehicleId,
    mountedTires,
    hasMountedTires,
    storageTires,
    disposedTires,
    loadTires,
  }
}
