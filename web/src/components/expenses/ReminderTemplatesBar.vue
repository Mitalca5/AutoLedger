<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { t } from '@/i18n'
import { api, type ReminderTemplate } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { Trash2 } from 'lucide-vue-next'

defineProps<{ hasReminders: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const vehicleStore = useVehicleStore()
const templates = ref<ReminderTemplate[]>([])
const selected = ref('')
const newName = ref('')
const message = ref('')
const error = ref('')

async function load() {
  try {
    templates.value = (await api.getReminderTemplates()).templates
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function apply() {
  const vehicle = vehicleStore.activeVehicle
  if (!vehicle || !selected.value) return
  error.value = ''
  try {
    const res = await api.applyReminderTemplate(vehicle.id, selected.value)
    message.value = t('expenses.remindersPanel.templates.applied', { created: res.created.length, skipped: res.skipped.length })
    emit('changed')
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function save() {
  const vehicle = vehicleStore.activeVehicle
  if (!vehicle || !newName.value.trim()) return
  error.value = ''
  try {
    await api.createReminderTemplate({ name: newName.value, from_vehicle_id: vehicle.id })
    newName.value = ''
    message.value = t('expenses.remindersPanel.templates.saved')
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : ''
  }
}

async function remove() {
  if (!selected.value) return
  await api.deleteReminderTemplate(selected.value)
  selected.value = ''
  await load()
}

onMounted(load)
</script>

<template>
  <details class="p-4 bg-slate-900 border border-slate-800 rounded-2xl text-xs">
    <summary class="font-bold text-white cursor-pointer">{{ $t('expenses.remindersPanel.templates.title') }}</summary>
    <div class="mt-3 space-y-3">
      <p class="text-slate-400">{{ $t('expenses.remindersPanel.templates.hint') }}</p>
      <div v-if="templates.length" class="flex flex-wrap gap-2 items-center">
        <select v-model="selected" :aria-label="$t('expenses.remindersPanel.templates.pick')" class="bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white">
          <option value="">{{ $t('expenses.remindersPanel.templates.pick') }}</option>
          <option v-for="tpl in templates" :key="tpl.id" :value="tpl.id">{{ tpl.name }} ({{ tpl.items.length }})</option>
        </select>
        <button type="button" class="px-3 py-1.5 rounded-xl bg-cyan-500/20 text-cyan-200 border border-cyan-500/30 font-semibold disabled:opacity-50" :disabled="!selected" @click="apply">
          {{ $t('expenses.remindersPanel.templates.apply') }}
        </button>
        <button type="button" class="text-slate-500 hover:text-rose-400 disabled:opacity-40" :disabled="!selected" :aria-label="$t('expenses.remindersPanel.templates.delete')" @click="remove">
          <Trash2 class="w-4 h-4" />
        </button>
      </div>
      <p v-else class="text-slate-500 italic">{{ $t('expenses.remindersPanel.templates.none') }}</p>
      <div v-if="hasReminders" class="flex flex-wrap gap-2 items-center">
        <input v-model="newName" maxlength="100" :placeholder="$t('expenses.remindersPanel.templates.saveName')" :aria-label="$t('expenses.remindersPanel.templates.saveName')" class="bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white" />
        <button type="button" class="px-3 py-1.5 rounded-xl bg-slate-800 text-slate-200 border border-slate-700 font-semibold disabled:opacity-50" :disabled="!newName.trim()" @click="save">
          {{ $t('expenses.remindersPanel.templates.save') }}
        </button>
      </div>
      <p v-if="message" class="text-emerald-400" role="status">{{ message }}</p>
      <p v-if="error" class="text-rose-400" role="alert">{{ error }}</p>
    </div>
  </details>
</template>
