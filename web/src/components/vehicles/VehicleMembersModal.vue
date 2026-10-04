<script setup lang="ts">
import { roleLabel } from '@/utils/vehicles'
import { t } from '@/i18n'
import { computed, ref, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { Trash2, RefreshCw, X, Users, UserPlus, ShieldCheck, LogOut } from 'lucide-vue-next'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

// Who can access a vehicle: the owner adds members and changes their role, a member can leave.
const props = defineProps<{ vehicle: any | null }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const vehicleStore = useVehicleStore()
const authStore = useAuthStore()
const { showConfirm, showAlert } = useConfirm()

const membersVehicle = computed(() => props.vehicle)
const members = ref<any[]>([])
const loadingMembers = ref(false)
const newMemberEmail = ref('')
const newMemberRole = ref<'EDITOR' | 'VIEWER'>('EDITOR')
const addingMember = ref(false)
const updatingMemberId = ref<string | null>(null)

watch(open, async (isOpen) => {
  if (!isOpen || !props.vehicle) return
  newMemberEmail.value = ''
  newMemberRole.value = 'EDITOR'
  await loadMembers(props.vehicle.id)
})

async function loadMembers(vehicleId: string) {
  loadingMembers.value = true
  try {
    members.value = await api.getVehicleMembers(vehicleId)
  } catch (err: any) {
    showAlert(t('vehicles.vehicleMembersModal.loadError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    loadingMembers.value = false
  }
}

async function handleAddMember() {
  if (!membersVehicle.value || !newMemberEmail.value.trim()) return
  addingMember.value = true
  try {
    await api.addVehicleMember(membersVehicle.value.id, {
      email: newMemberEmail.value.trim(),
      role: newMemberRole.value,
    })
    newMemberEmail.value = ''
    await loadMembers(membersVehicle.value.id)
    showAlert(t('vehicles.vehicleMembersModal.added'), t('common.success'), 'info')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    addingMember.value = false
  }
}

async function handleUpdateMemberRole(m: any, newRole: string) {
  if (!membersVehicle.value || m.role === newRole) return
  updatingMemberId.value = m.user_id
  try {
    await api.updateVehicleMemberRole(membersVehicle.value.id, m.user_id, { role: newRole })
    await loadMembers(membersVehicle.value.id)
    showAlert(t('vehicles.vehicleMembersModal.roleUpdated'), t('common.success'), 'info')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    updatingMemberId.value = null
  }
}

async function handleRemoveMember(m: any) {
  if (!membersVehicle.value) return
  const isSelf = authStore.user?.id === m.user_id
  const ok = await showConfirm({
    title: isSelf ? t('vehicles.vehicleMembersModal.leaveTitle') : t('vehicles.vehicleMembersModal.removeTitle'),
    message: isSelf
      ? t('vehicles.vehicleMembersModal.leaveMessage', { name: membersVehicle.value.name })
      : t('vehicles.vehicleMembersModal.removeMessage', { email: m.user_email }),
    confirmText: isSelf ? t('vehicles.vehicleMembersModal.leave') : t('vehicles.vehicleMembersModal.remove'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.removeVehicleMember(membersVehicle.value.id, m.user_id)
    if (isSelf) {
      open.value = false
      await vehicleStore.fetchVehicles()
    } else {
      await loadMembers(membersVehicle.value.id)
    }
    showAlert(isSelf ? t('vehicles.vehicleMembersModal.left') : t('vehicles.vehicleMembersModal.revoked'), t('common.success'), 'info')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden">
      <!-- Header -->
      <div class="px-5 py-4 border-b border-slate-800 flex items-center justify-between shrink-0">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-violet-500/10 text-violet-400 rounded-xl">
            <Users class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-base font-bold text-white">{{ $t('vehicles.vehicleMembersModal.sharingAndAccess') }}</h3>
            <p class="text-xs text-slate-400">{{ membersVehicle?.name }}</p>
          </div>
        </div>
        <button
          @click="open = false"
          class="tap p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition-colors"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto space-y-6 text-xs">
        <!-- Add Member Section (Owners only) -->
        <div
          v-if="membersVehicle?.role === 'OWNER'"
          class="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-3"
        >
          <div class="flex items-center gap-2">
            <UserPlus class="w-4 h-4 text-violet-400" />
            <h4 class="text-xs font-bold text-white uppercase tracking-wider">{{ $t('vehicles.vehicleMembersModal.addAMember') }}</h4>
          </div>
          <p class="text-xs text-slate-400">
            {{ $t('vehicles.vehicleMembersModal.inviteACoDriverOr') }}
          </p>

          <form @submit.prevent="handleAddMember" class="space-y-3">
            <div class="grid grid-cols-1 sm:grid-cols-12 gap-2">
              <div class="sm:col-span-7">
                <label for="new-member-email" class="sr-only">{{ $t('vehicles.vehicleMembersModal.memberEmail') }}</label>
                <input
                  id="new-member-email"
                  v-model="newMemberEmail"
                  type="email"
                  required
                  :placeholder="$t('vehicles.vehicleMembersModal.emailExampleCom')"
                  class="field placeholder-slate-500 focus:border-violet-500"
                />
              </div>
              <div class="sm:col-span-5">
                <label for="new-member-role" class="sr-only">{{ $t('vehicles.vehicleMembersModal.memberRole') }}</label>
                <select
                  id="new-member-role"
                  v-model="newMemberRole"
                  class="field focus:border-violet-500"
                >
                  <option value="EDITOR">{{ $t('vehicles.vehicleMembersModal.coDriverEditor') }}</option>
                  <option value="VIEWER">{{ $t('vehicles.vehicleMembersModal.readOnly') }}</option>
                </select>
              </div>
            </div>

            <div class="flex items-center justify-between gap-3 pt-1">
              <p class="text-xs text-slate-400 leading-tight">
                <ShieldCheck class="w-3 h-3 text-success-400 inline mr-0.5 -mt-0.5" />
                {{ $t('vehicles.vehicleMembersModal.yourTeslamateApiKeysAnd') }}
              </p>
              <button
                type="submit"
                :disabled="addingMember || !newMemberEmail.trim()"
                class="px-3 py-1.5 bg-violet-600 hover:bg-violet-500 disabled:opacity-50 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shrink-0 transition-colors shadow-lg shadow-violet-600/20"
              >
                <RefreshCw v-if="addingMember" class="w-3.5 h-3.5 animate-spin" />
                <UserPlus v-else class="w-3.5 h-3.5" />
                <span>{{ $t('vehicles.vehicleMembersModal.add') }}</span>
              </button>
            </div>
          </form>
        </div>

        <!-- Members List -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-white uppercase tracking-wider">{{ $t('vehicles.vehicleMembersModal.authorizedMembers') }}</h4>
            <span class="text-xs text-slate-400">{{ $t('vehicles.vehicleMembersModal.memberS', { length: members.length }) }}</span>
          </div>

          <div v-if="loadingMembers" class="py-8 text-center text-xs text-slate-400">
            {{ $t('vehicles.vehicleMembersModal.loadingTheAccess') }}
          </div>

          <div v-else-if="members.length === 0" class="py-6 text-center text-xs text-slate-400 bg-slate-950/40 rounded-xl border border-slate-800">
            {{ $t('vehicles.vehicleMembersModal.noMemberFound') }}
          </div>

          <div v-else class="space-y-2">
            <div
              v-for="m in members"
              :key="m.user_id"
              class="bg-slate-950/60 border border-slate-800 rounded-xl p-3 flex items-center justify-between gap-3"
            >
              <div class="flex items-center gap-3 min-w-0">
                <div
                  class="w-8 h-8 rounded-full flex items-center justify-center font-bold text-xs shrink-0"
                  :class="m.role === 'OWNER' ? 'bg-warning-500/10 text-warning-400 border border-warning-500/20' : 'bg-slate-800 text-slate-300 border border-transparent'"
                >
                  {{ (m.user_email || '?').charAt(0).toUpperCase() }}
                </div>
                <div class="min-w-0">
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="text-xs font-semibold text-white truncate">{{ m.user_email }}</span>
                    <span
                      v-if="m.user_id === authStore.user?.id"
                      class="text-xs px-1.5 py-0.2 bg-slate-800 text-slate-400 rounded"
                    >
                      {{ $t('vehicles.vehicleMembersModal.you') }}
                    </span>
                  </div>
                  <div class="flex items-center gap-1.5 mt-0.5">
                    <span
                      class="text-xs px-2 py-0.5 rounded-full font-semibold uppercase tracking-wider"
                      :class="{
                        'bg-warning-500/10 text-warning-400 border border-warning-500/20': m.role === 'OWNER',
                        'bg-info-500/10 text-info-400 border border-info-500/20': m.role === 'EDITOR',
                        'bg-slate-800 text-slate-400 border border-slate-700': m.role === 'VIEWER',
                      }"
                    >
                      {{ roleLabel(m.role) }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- Member Management Actions -->
              <div class="flex items-center gap-2 shrink-0">
                <!-- If current user is OWNER and this member is not OWNER: allow role change or removal -->
                <template v-if="membersVehicle?.role === 'OWNER' && m.role !== 'OWNER'">
                  <label :for="'member-role-' + m.user_id" class="sr-only">{{ $t('vehicles.vehicleMembersModal.roleOfTheMember', { user_email: m.user_email }) }}</label>
                  <select
                    :id="'member-role-' + m.user_id"
                    :value="m.role"
                    :disabled="updatingMemberId === m.user_id"
                    @change="handleUpdateMemberRole(m, ($event.target as HTMLSelectElement).value)"
                    class="field text-slate-200 focus:border-violet-500"
                  >
                    <option value="EDITOR">{{ $t('vehicles.vehicleMembersModal.coDriver') }}</option>
                    <option value="VIEWER">{{ $t('vehicles.vehicleMembersModal.viewer') }}</option>
                  </select>

                  <button
                    type="button"
                    @click="handleRemoveMember(m)"
                    class="tap p-1.5 text-slate-400 hover:text-rose-400 hover:bg-slate-800 rounded-lg transition-colors"
                    :title="$t('vehicles.vehicleMembersModal.removeAccess')"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                </template>

                <!-- If current user is non-owner and viewing themselves: allow leaving -->
                <template v-else-if="membersVehicle?.role !== 'OWNER' && m.user_id === authStore.user?.id">
                  <button
                    type="button"
                    @click="handleRemoveMember(m)"
                    class="px-2.5 py-1 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 text-xs font-semibold rounded-lg flex items-center gap-1.5 transition-colors border border-rose-500/20"
                  >
                    <LogOut class="w-3.5 h-3.5" />
                    <span>{{ $t('vehicles.vehicleMembersModal.leave') }}</span>
                  </button>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="px-5 py-3.5 border-t border-slate-800 flex justify-end shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
        >
          {{ $t('common.close') }}
        </button>
      </div>
    </div>
  </div>
</template>
