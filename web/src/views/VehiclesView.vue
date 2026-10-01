<script setup lang="ts">
import { t } from '@/i18n'
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import VehicleCard from '@/components/vehicles/VehicleCard.vue'
import VehicleFormModal from '@/components/vehicles/VehicleFormModal.vue'
import OwnershipWizardModal from '@/components/vehicles/OwnershipWizardModal.vue'
import VehicleMembersModal from '@/components/vehicles/VehicleMembersModal.vue'
import { Plus } from 'lucide-vue-next'

// The page owns the vehicle list, the acquisition contracts and which modal is open; each modal owns its form
// and its API calls and reports back.
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const route = useRoute()
const router = useRouter()

// Add / edit vehicle
const showModal = ref(false)
const editingVehicle = ref<any | null>(null)

// Ownership contracts (purchase, loan, LOA, LLD) by vehicle id
const ownerships = ref<Record<string, any | null>>({})
const showOwnershipModal = ref(false)
const ownershipVehicle = ref<any | null>(null)

// Shared vehicle members
const showMembersModal = ref(false)
const membersVehicle = ref<any | null>(null)

async function loadOwnerships() {
  const entries = await Promise.all(
    vehicleStore.vehicles.map(async (v) => {
      try {
        if (v.role && v.role !== 'OWNER') return [v.id, null] as const
        return [v.id, await api.getOwnership(v.id)] as const
      } catch {
        return [v.id, null] as const
      }
    })
  )
  ownerships.value = Object.fromEntries(entries)
}

function checkAddQuery() {
  if (route.query.add === '1' || route.query.add === 'true') {
    openCreateModal()
    router.replace({ path: '/vehicles', query: {} })
  }
}

onMounted(async () => {
  await vehicleStore.fetchVehicles()
  await loadOwnerships()
  checkAddQuery()
})

watch(() => route.query.add, () => {
  checkAddQuery()
})

function openCreateModal() {
  editingVehicle.value = null
  showModal.value = true
}

function openEditModal(v: any) {
  editingVehicle.value = v
  showModal.value = true
}

async function onVehicleSaved() {
  await vehicleStore.fetchVehicles()
  await loadOwnerships()
}

async function handleDelete(id: string) {
  const ok = await showConfirm({
    title: t('vehicles.vehiclesView.deleteTitle'),
    message: t('vehicles.vehiclesView.deleteMessage'),
    confirmText: t('common.deleteForever'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteVehicle(id)
    await vehicleStore.fetchVehicles()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

function openOwnershipModal(v: any) {
  ownershipVehicle.value = v
  showOwnershipModal.value = true
}

// A saved or deleted contract changes the TCO: nudge the pages that reload on sync
function onOwnershipSaved(ownership: any) {
  if (!ownershipVehicle.value) return
  ownerships.value[ownershipVehicle.value.id] = ownership
  vehicleStore.lastSyncTimestamp = Date.now()
}

function onOwnershipDeleted() {
  if (!ownershipVehicle.value) return
  ownerships.value[ownershipVehicle.value.id] = null
  vehicleStore.lastSyncTimestamp = Date.now()
}

function openMembersModal(v: any) {
  membersVehicle.value = v
  showMembersModal.value = true
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">{{ $t('vehicles.vehiclesView.vehicleManagement') }}</h2>
        <p class="text-sm text-slate-400">{{ $t('vehicles.vehiclesView.setUpYourVehiclesAnd') }}</p>
      </div>

      <button
        @click="openCreateModal"
        class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
      >
        <Plus class="w-3.5 h-3.5" />
        {{ $t('vehicles.vehiclesView.addAVehicle') }}
      </button>
    </div>

    <!-- Vehicles Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <VehicleCard
        v-for="v in vehicleStore.vehicles"
        :key="v.id"
        :v="v"
        :ownership="ownerships[v.id]"
        @edit="openEditModal"
        @delete="handleDelete"
        @members="openMembersModal"
        @ownership="openOwnershipModal"
      />
    </div>

    <VehicleFormModal v-model:open="showModal" :editing="editingVehicle" :vehicles="vehicleStore.vehicles" @saved="onVehicleSaved" />

    <OwnershipWizardModal
      v-model:open="showOwnershipModal"
      :vehicle="ownershipVehicle"
      :ownership="ownershipVehicle ? ownerships[ownershipVehicle.id] : null"
      @saved="onOwnershipSaved"
      @deleted="onOwnershipDeleted"
    />

    <VehicleMembersModal v-model:open="showMembersModal" :vehicle="membersVehicle" />
  </div>
</template>
