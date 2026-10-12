<script setup lang="ts">
import { ref, watch } from 'vue'
import { MapPin, Trash2 } from 'lucide-vue-next'
import ModalShell from '@/components/ModalShell.vue'
import { api } from '@/services/api'
import { apiErrorMessage } from '@/services/apiError'
import { useConfirm } from '@/composables/useConfirm'
import { useSubmit } from '@/composables/useSubmit'
import { formatNumber } from '@/utils/numbers'
import { PLACE_CATEGORIES, placeCategoryKey, type ChargingLocation, type ChargingPlace } from '@/utils/chargingPlaces'
import { t } from '@/i18n'

// Places of the account and the classification of the locations the charges report.
const props = defineProps<{ vehicleId: string; canEdit: boolean }>()
const emit = defineEmits<{ changed: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showConfirm, showAlert } = useConfirm()

const places = ref<ChargingPlace[]>([])
const locations = ref<ChargingLocation[]>([])
const loading = ref(false)
const error = ref('')
const form = ref({ name: '', category: 'HOME' as string })
const { pending: submitting, run: runOnce } = useSubmit()

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getChargingPlaces(props.vehicleId)
    places.value = res.places || []
    locations.value = res.locations || []
  } catch (e: any) {
    error.value = apiErrorMessage(e, t('expenses.chargingPlaces.failed'))
  } finally {
    loading.value = false
  }
}

watch(open, (isOpen) => {
  if (isOpen) void load()
})

async function mutate(action: () => Promise<unknown>) {
  try {
    await action()
    await load()
    emit('changed')
  } catch (e: any) {
    void showAlert(apiErrorMessage(e, t('expenses.chargingPlaces.save')), t('shell.confirm.error'), 'danger')
  }
}

const addPlace = () =>
  runOnce(async () => {
    const name = form.value.name.trim()
    if (!name) return
    await mutate(async () => {
      await api.createChargingPlace(props.vehicleId, { name, category: form.value.category })
      form.value.name = ''
    })
  })

function assign(location: ChargingLocation, placeId: string) {
  return mutate(() => api.setChargingPlaceAlias(props.vehicleId, location.text, placeId || null))
}

async function removePlace(place: ChargingPlace) {
  const ok = await showConfirm({ message: t('expenses.chargingPlaces.deleteConfirm', { name: place.name }), type: 'danger' })
  if (ok) await mutate(() => api.deleteChargingPlace(props.vehicleId, place.id))
}
</script>

<template>
  <ModalShell v-model:open="open" size="lg" :title="$t('expenses.chargingPlaces.title')" :icon="MapPin">
    <p class="text-xs text-slate-400">{{ $t('expenses.chargingPlaces.intro') }}</p>
    <p v-if="error" class="text-xs text-danger-400" role="alert">{{ error }}</p>

    <section class="space-y-2">
      <h4 class="text-xs font-bold text-white">{{ $t('expenses.chargingPlaces.locations') }}</h4>
      <p v-if="!loading && !locations.length" class="text-xs text-slate-400">{{ $t('expenses.chargingPlaces.noLocations') }}</p>
      <ul v-else class="space-y-1.5 max-h-72 overflow-y-auto pr-1">
        <li v-for="l in locations" :key="l.text" class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-2.5 bg-slate-800/50 border border-slate-700/40 rounded-xl text-xs">
          <div class="min-w-0">
            <p class="font-semibold text-white truncate" :title="l.text">{{ l.text }}</p>
            <p class="text-slate-400">{{ $t('expenses.chargingPlaces.sessions', { count: l.sessions }) }} · {{ formatNumber(l.kwh_added, 1) }} kWh</p>
          </div>
          <select
            class="field sm:w-52"
            :value="l.place_id || ''"
            :disabled="!canEdit"
            :aria-label="$t('expenses.chargingPlaces.assign', { text: l.text })"
            @change="assign(l, ($event.target as HTMLSelectElement).value)"
          >
            <option value="">{{ $t('expenses.chargingPlaces.unclassified') }}</option>
            <option v-for="p in places" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </li>
      </ul>
    </section>

    <section class="space-y-2">
      <h4 class="text-xs font-bold text-white">{{ $t('expenses.chargingPlaces.yourPlaces') }}</h4>
      <ul class="space-y-1.5">
        <li v-for="p in places" :key="p.id" class="flex items-center justify-between gap-2 p-2 bg-slate-800/40 border border-slate-700/40 rounded-xl text-xs">
          <span class="min-w-0 truncate"><span class="font-semibold text-white">{{ p.name }}</span> <span class="text-slate-400">· {{ $t(placeCategoryKey(p.category)) }}</span></span>
          <button
            v-if="canEdit"
            type="button"
            class="tap p-1.5 text-rose-400/80 hover:text-rose-300 rounded-lg"
            :title="$t('expenses.chargingPlaces.delete')"
            :aria-label="$t('expenses.chargingPlaces.delete')"
            @click="removePlace(p)"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </li>
      </ul>
      <form v-if="canEdit" class="grid grid-cols-1 sm:grid-cols-[1fr_auto_auto] gap-2 items-end" @submit.prevent="addPlace">
        <div>
          <label for="place-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargingPlaces.name') }}</label>
          <input id="place-name" v-model="form.name" class="field" maxlength="100" required :placeholder="$t('expenses.chargingPlaces.namePlaceholder')" />
        </div>
        <div>
          <label for="place-category" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.chargingPlaces.categoryLabel') }}</label>
          <select id="place-category" v-model="form.category" class="field">
            <option v-for="c in PLACE_CATEGORIES" :key="c" :value="c">{{ $t(placeCategoryKey(c)) }}</option>
          </select>
        </div>
        <button type="submit" :disabled="submitting" class="btn btn-primary">{{ $t('expenses.chargingPlaces.add') }}</button>
      </form>
    </section>
  </ModalShell>
</template>
