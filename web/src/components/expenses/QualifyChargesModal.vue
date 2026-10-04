<script setup lang="ts">
import { formatNumber, formatPercent } from '@/utils/numbers'
import { ref, onMounted, computed, watch } from 'vue'
import { Zap, Check, Trash2, X, Car, Calendar, User, ArrowRight, Clock } from 'lucide-vue-next'
import { api, type PendingCharge } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { formatDate } from '@/utils/expenses'
import { t } from '@/i18n'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

const vehicleStore = useVehicleStore()

const emit = defineEmits<{
  assigned: []
}>()

const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))

const pendingCharges = ref<PendingCharge[]>([])
const loading = ref(false)
const assigningId = ref<string | null>(null)
const dismissingId = ref<string | null>(null)

// Vehicle selection per pending charge
const selectedVehicleIds = ref<Record<string, string>>({})

async function loadPendingCharges() {
  loading.value = true
  try {
    const res = await api.getPendingCharges()
    pendingCharges.value = res.pending_charges || []
    // Initialize default vehicle selection
    const defaultVehId = vehicleStore.activeVehicle?.id || vehicleStore.vehicles[0]?.id || ''
    for (const c of pendingCharges.value) {
      if (!selectedVehicleIds.value[c.id]) {
        selectedVehicleIds.value[c.id] = defaultVehId
      }
    }
  } catch (err) {
    console.error('Failed to load pending charges', err)
  } finally {
    loading.value = false
  }
}

watch(open, (isOpen) => {
  if (isOpen) {
    loadPendingCharges()
  }
})

async function handleAssign(charge: PendingCharge) {
  const targetVehId = selectedVehicleIds.value[charge.id]
  if (!targetVehId) return

  assigningId.value = charge.id
  try {
    await api.assignPendingCharge(charge.id, {
      vehicle_id: targetVehId,
    })
    pendingCharges.value = pendingCharges.value.filter((c) => c.id !== charge.id)
    emit('assigned')
    if (pendingCharges.value.length === 0) {
      open.value = false
    }
  } catch (err) {
    console.error('Failed to assign pending charge', err)
  } finally {
    assigningId.value = null
  }
}

async function handleDismiss(charge: PendingCharge) {
  dismissingId.value = charge.id
  try {
    await api.deletePendingCharge(charge.id)
    pendingCharges.value = pendingCharges.value.filter((c) => c.id !== charge.id)
    if (pendingCharges.value.length === 0) {
      open.value = false
    }
  } catch (err) {
    console.error('Failed to dismiss pending charge', err)
  } finally {
    dismissingId.value = null
  }
}

onMounted(() => {
  if (open.value) {
    loadPendingCharges()
  }
})
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm" @click.self="open = false">
    <div v-dialog class="w-full max-w-2xl bg-slate-900 border border-slate-800 rounded-2xl shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
      <!-- Header -->
      <div class="flex items-center justify-between p-4 border-b border-slate-800">
        <div class="flex items-center gap-2">
          <div class="p-2 bg-warning-500/10 text-warning-400 rounded-lg">
            <Zap class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-sm font-bold text-white">{{ t('pendingCharges.modalTitle') }}</h2>
            <p class="text-xs text-slate-400">{{ t('pendingCharges.modalSubtitle') }}</p>
          </div>
        </div>
        <button @click="open = false" class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800">
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Content -->
      <div class="p-5 overflow-y-auto space-y-4">
        <div v-if="loading" class="text-center py-8 text-sm text-slate-400">
          {{ t('pendingCharges.loading') }}
        </div>

        <div v-else-if="pendingCharges.length === 0" class="text-center py-8 text-xs text-slate-400">
          {{ t('pendingCharges.empty') }}
        </div>

        <div v-else class="space-y-3">
          <div
            v-for="charge in pendingCharges"
            :key="charge.id"
            class="p-4 rounded-xl bg-slate-950/70 border border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4"
          >
            <!-- Charge Details -->
            <div class="space-y-1 min-w-0">
              <div class="flex items-center gap-2">
                <span class="font-bold text-sm text-white">
                  {{ formatNumber(charge.energy_kwh, 2) }} kWh
                </span>
                <span v-if="charge.charger_name" class="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 font-medium">
                  {{ charge.charger_name }}
                </span>
                <span v-if="charge.location" class="text-xs text-slate-400">
                  {{ charge.location }}
                </span>
              </div>

              <div class="flex items-center gap-3 text-xs text-slate-400">
                <span class="flex items-center gap-1">
                  <Calendar class="w-3 h-3 text-slate-400" />
                  {{ formatDate(charge.start_time) }}
                </span>
              </div>
            </div>

            <!-- Vehicle Assignment Action -->
            <div class="flex items-center gap-2 shrink-0">
              <label :for="'qualify-vehicle-' + charge.id" class="sr-only">
                {{ t('pendingCharges.selectVehicle') }}
              </label>
              <select
                :id="'qualify-vehicle-' + charge.id"
                v-model="selectedVehicleIds[charge.id]"
                class="field focus:border-warning-500"
              >
                <option v-for="v in vehicleStore.vehicles" :key="v.id" :value="v.id">
                  {{ v.name }}
                </option>
              </select>

              <button
                type="button"
                :disabled="assigningId === charge.id || !selectedVehicleIds[charge.id]"
                @click="handleAssign(charge)"
                class="inline-flex items-center gap-1 px-3 py-1.5 rounded-xl bg-warning-600 hover:bg-warning-500 text-white text-xs font-semibold transition-colors disabled:opacity-50"
              >
                <Check class="w-3.5 h-3.5" />
                <span>{{ assigningId === charge.id ? t('pendingCharges.assigning') : t('pendingCharges.assign') }}</span>
              </button>

              <button
                type="button"
                :disabled="dismissingId === charge.id"
                @click="handleDismiss(charge)"
                class="p-1.5 rounded-xl text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors"
                :title="t('pendingCharges.dismiss')"
              >
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="p-3 border-t border-slate-800 bg-slate-950/40 flex justify-end">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 rounded-xl text-xs font-semibold text-slate-300 hover:bg-slate-800"
        >
          {{ t('common.close') }}
        </button>
      </div>
    </div>
  </div>
</template>
