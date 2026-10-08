<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CalendarPlus, Copy, Pencil, Plus, Trash2, Zap } from 'lucide-vue-next'
import { api, type TariffPlan } from '@/services/api'
import { t } from '@/i18n'
import { formatAmount } from '@/currency'
import { formatCalendarDay } from '@/utils/expenses'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import LoadError from '@/components/LoadError.vue'
import TariffPlanModal from '@/components/tariffs/TariffPlanModal.vue'
import { groupVersions, nextVersionForm, planToForm, type PlanForm } from '@/utils/tariffPlans'

const { showConfirm } = useConfirm()
const vehicleStore = useVehicleStore()

const plans = ref<TariffPlan[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)
const modalOpen = ref(false)
const editingId = ref<string | null>(null)
const seed = ref<PlanForm | null>(null)

const groups = computed(() => groupVersions(plans.value))

async function load() {
  loading.value = true
  loadError.value = null
  try {
    plans.value = (await api.getTariffPlans()).plans ?? []
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : ''
  } finally {
    loading.value = false
  }
}

function openModal(id: string | null, form: PlanForm | null) {
  editingId.value = id
  seed.value = form
  modalOpen.value = true
}

function rateSummary(p: TariffPlan): string {
  const cur = p.currency
  if (p.plan_type === 'FLAT') return p.flat_rate_cents == null ? '' : t('tariffs.editor.perKwh', { amount: formatAmount(p.flat_rate_cents, cur) })
  return (p.bands ?? []).map((b) => `${b.name} ${formatAmount(b.rate_cents, cur)}`).join(' · ')
}

function validity(p: TariffPlan): string {
  if (p.valid_from && p.valid_to) return t('tariffs.editor.validBetween', { from: formatCalendarDay(p.valid_from), to: formatCalendarDay(p.valid_to) })
  if (p.valid_from) return t('tariffs.editor.validSince', { from: formatCalendarDay(p.valid_from) })
  if (p.valid_to) return t('tariffs.editor.validUntil', { to: formatCalendarDay(p.valid_to) })
  return t('tariffs.editor.validAlways')
}

async function remove(p: TariffPlan) {
  const ok = await showConfirm({
    title: t('tariffs.editor.deleteTitle'),
    message: t('tariffs.editor.deleteMessage', { name: p.name }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteTariffPlan(p.id)
    plans.value = plans.value.filter((x) => x.id !== p.id)
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : ''
  }
}

onMounted(load)
</script>

<template>
  <section class="rounded-2xl border border-slate-800 bg-slate-900 p-5 space-y-4" aria-labelledby="account-tariffs">
    <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 pb-3">
      <div>
        <h2 id="account-tariffs" class="flex items-center gap-2 text-sm font-bold text-white">
          <Zap class="h-4 w-4 text-warning-400" aria-hidden="true" />
          {{ $t('tariffs.editor.title') }}
        </h2>
        <p class="text-xs text-slate-400 mt-0.5">{{ $t('tariffs.editor.subtitle') }}</p>
      </div>
      <button type="button" class="btn btn-primary" @click="openModal(null, null)">
        <Plus class="h-3.5 w-3.5" />
        {{ $t('tariffs.editor.create') }}
      </button>
    </div>

    <div v-if="loading" class="text-sm text-slate-400 py-2">{{ $t('common.loading') }}</div>
    <LoadError v-else-if="loadError !== null" :message="loadError" @retry="load" />
    <p v-else-if="!groups.length" class="text-center py-6 text-xs text-slate-400">{{ $t('tariffs.editor.empty') }}</p>
    <ul v-else class="space-y-3">
      <li v-for="g in groups" :key="g.name" class="rounded-xl border border-slate-800 bg-slate-950/60 p-3 space-y-2">
        <h3 class="text-sm font-semibold text-white">{{ g.name }}</h3>
        <ul class="divide-y divide-slate-800">
          <li v-for="p in g.versions" :key="p.id" class="flex items-center justify-between gap-3 py-2 first:pt-0 last:pb-0">
            <div class="min-w-0">
              <p class="text-xs text-slate-300">
                {{ validity(p) }}
                <span v-if="p.is_default" class="ml-1 rounded bg-primary-500/20 px-1.5 py-0.5 text-[11px] text-primary-200">{{ $t('tariffs.editor.defaultBadge') }}</span>
              </p>
              <p class="text-xs text-slate-400 truncate">{{ rateSummary(p) }}</p>
            </div>
            <div class="flex shrink-0 items-center">
              <button type="button" class="tap p-2 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800" :title="$t('common.edit')" :aria-label="$t('tariffs.editor.editPlan', { name: g.name })" @click="openModal(p.id, planToForm(p))">
                <Pencil class="h-4 w-4" />
              </button>
              <button type="button" class="tap p-2 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800" :title="$t('tariffs.editor.newVersion')" :aria-label="$t('tariffs.editor.newVersionOf', { name: g.name })" @click="openModal(null, nextVersionForm(p))">
                <CalendarPlus class="h-4 w-4" />
              </button>
              <button type="button" class="tap p-2 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800" :title="$t('tariffs.editor.duplicate')" :aria-label="$t('tariffs.editor.duplicateOf', { name: g.name })" @click="openModal(null, { ...planToForm(p), name: '', isDefault: false })">
                <Copy class="h-4 w-4" />
              </button>
              <button type="button" class="tap p-2 rounded-lg text-slate-400 hover:text-danger-400 hover:bg-danger-500/10" :title="$t('common.delete')" :aria-label="$t('tariffs.editor.deletePlan', { name: g.name })" @click="remove(p)">
                <Trash2 class="h-4 w-4" />
              </button>
            </div>
          </li>
        </ul>
      </li>
    </ul>

    <TariffPlanModal v-model:open="modalOpen" :plan-id="editingId" :seed="seed" :currency="vehicleStore.currency" @saved="load" />
  </section>
</template>
