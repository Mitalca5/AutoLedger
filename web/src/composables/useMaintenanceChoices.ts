import { computed, ref, type Ref } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { categoryLabel, formatDate, isFixedCost, sortMaintenanceByDate } from '@/utils/expenses'
import { distanceUnit, formatDistanceValue } from '@/units'

/** Recorded maintenance a reminder can be based on or closed with: loaded on demand, newest first, fixed costs left out. */
export function useMaintenanceChoices(vehicleId: () => string) {
  const choices: Ref<any[]> = ref([])
  const options = computed(() => sortMaintenanceByDate(choices.value.filter((m) => !isFixedCost(m.category))))

  async function load() {
    try {
      choices.value = vehicleId() ? await api.getMaintenance(vehicleId()) : []
    } catch {
      choices.value = []
    }
  }

  function label(m: any): string {
    const km = m.odometer != null ? ` · ${t('common.atKm', { unit: distanceUnit(), km: formatDistanceValue(m.odometer) })}` : ''
    return `${formatDate(m.date)} · ${m.description || categoryLabel(m.category)}${km}`
  }

  const find = (id: string) => choices.value.find((x) => x.id === id)

  return { choices, options, load, label, find }
}
