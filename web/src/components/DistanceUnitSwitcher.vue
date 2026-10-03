<script setup lang="ts">
import { computed } from 'vue'
import { Ruler } from 'lucide-vue-next'
import { t } from '@/i18n'
import { currentDistanceUnit, setDistanceUnit, SUPPORTED_DISTANCE_UNITS, type DistanceUnit } from '@/units'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'

// Called from the template (not precomputed) so the label follows a later language change.
const unitLabel = (u: DistanceUnit) => (u === 'mi' ? t('account.distanceUnitMi') : t('account.distanceUnitKm'))

const authStore = useAuthStore()

const selected = computed({
  get: () => currentDistanceUnit(),
  set: (value: DistanceUnit) => {
    setDistanceUnit(value)
    if (authStore.isAuthenticated) api.updateDistanceUnit(value).catch(() => {})
  },
})
</script>

<template>
  <label class="flex items-center gap-1.5 text-xs text-slate-400">
    <Ruler class="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
    <span class="sr-only">{{ $t('account.distanceUnit') }}</span>
    <select
      v-model="selected"
      class="bg-slate-900 border border-slate-700 rounded-md px-1.5 py-0.5 text-xs text-slate-200 focus:outline-none focus:border-rose-500"
    >
      <option v-for="u in SUPPORTED_DISTANCE_UNITS" :key="u" :value="u">{{ unitLabel(u) }}</option>
    </select>
  </label>
</template>
