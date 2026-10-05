import { defineStore } from 'pinia'
import { ref } from 'vue'
import { monthlyRangeOptions, type MonthlyRangeKey } from '@/utils/dashboard'

const PRO_PERSO_KEY = 'teslacost_pro_perso'
const DASHBOARD_RANGE_KEY = 'teslacost_dashboard_range'
const DEFAULT_DASHBOARD_RANGE: MonthlyRangeKey = '1Y'

const storage = () => (typeof localStorage === 'undefined' ? null : localStorage)

// Display preferences of the person using this browser (like the language), not data of the vehicle.
export const usePreferencesStore = defineStore('preferences', () => {
  // Work / personal classification of drives; on by default
  const proPersoEnabled = ref(storage()?.getItem(PRO_PERSO_KEY) !== 'false')

  function setProPersoEnabled(enabled: boolean) {
    proPersoEnabled.value = enabled
    storage()?.setItem(PRO_PERSO_KEY, String(enabled))
  }

  // Period shared by the dashboard's monthly charts
  const storedRange = storage()?.getItem(DASHBOARD_RANGE_KEY)
  const dashboardRange = ref<MonthlyRangeKey>(
    monthlyRangeOptions.some((o) => o.key === storedRange) ? (storedRange as MonthlyRangeKey) : DEFAULT_DASHBOARD_RANGE
  )

  function setDashboardRange(range: MonthlyRangeKey) {
    dashboardRange.value = range
    storage()?.setItem(DASHBOARD_RANGE_KEY, range)
  }

  return { proPersoEnabled, setProPersoEnabled, dashboardRange, setDashboardRange }
})
