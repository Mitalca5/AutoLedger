<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Plus, Trash2, X, Zap } from 'lucide-vue-next'
import { api, type TariffPlan } from '@/services/api'
import { intlLocale, t } from '@/i18n'
import { currencySymbol } from '@/currency'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import NumberInput from '@/components/NumberInput.vue'
import {
  WEEK_ORDER,
  emptyPlanForm,
  formToPayload,
  planProblem,
  removeBand,
  renameBand,
  type PlanForm,
} from '@/utils/tariffPlans'

// Creates or edits one tariff plan (one version of a tariff); `seed` carries the values to start from.
const props = defineProps<{ planId: string | null; seed: PlanForm | null; currency: string }>()
const emit = defineEmits<{ (e: 'saved', plan: TariffPlan): void }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))

const form = ref<PlanForm>(emptyPlanForm(props.currency))
const saving = ref(false)
const error = ref('')

watch(open, (isOpen) => {
  if (!isOpen) return
  form.value = props.seed ? JSON.parse(JSON.stringify(props.seed)) : emptyPlanForm(props.currency)
  error.value = ''
})

const symbol = computed(() => currencySymbol(form.value.currency))
const problem = computed(() => planProblem(form.value))
const problemText = computed(() => (problem.value ? t(`tariffs.editor.problem.${problem.value}`) : ''))
const namedBands = computed(() => form.value.bands.map((b) => b.name.trim()).filter(Boolean))
const dayLabel = (d: number) => new Date(Date.UTC(2024, 0, 7 + d)).toLocaleDateString(intlLocale(), { weekday: 'short', timeZone: 'UTC' })

function addBand() {
  form.value.bands.push({ name: '', rate: null })
}

function addRule() {
  form.value.rules.push({ days: [], start: '22:00', end: '06:00', band: namedBands.value[1] ?? namedBands.value[0] ?? '' })
}

function toggleDay(rule: PlanForm['rules'][number], day: number) {
  rule.days = rule.days.includes(day) ? rule.days.filter((d) => d !== day) : [...rule.days, day]
}

async function save() {
  if (problem.value) return
  saving.value = true
  error.value = ''
  try {
    const payload = formToPayload(form.value)
    const saved = props.planId ? await api.updateTariffPlan(props.planId, payload) : await api.createTariffPlan(payload)
    emit('saved', saved)
    open.value = false
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('tariffs.editor.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
    <form
      v-dialog
      class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-2xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden"
      :aria-label="planId ? $t('tariffs.editor.editTitle') : $t('tariffs.editor.newTitle')"
      @submit.prevent="save"
    >
      <div class="px-5 py-4 border-b border-slate-800 flex items-center justify-between shrink-0">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-warning-500/10 text-warning-400 rounded-xl"><Zap class="w-5 h-5" /></div>
          <h3 class="text-base font-bold text-white">{{ planId ? $t('tariffs.editor.editTitle') : $t('tariffs.editor.newTitle') }}</h3>
        </div>
        <button type="button" class="tap p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800" :aria-label="$t('common.close')" @click="open = false">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto p-5 space-y-5">
        <div class="grid sm:grid-cols-2 gap-3">
          <div>
            <label for="tp-name" class="field-label">{{ $t('tariffs.editor.name') }}</label>
            <input id="tp-name" v-model="form.name" maxlength="100" class="field" :placeholder="$t('tariffs.editor.namePlaceholder')" />
            <p class="mt-1 text-xs text-slate-400">{{ $t('tariffs.editor.nameHint') }}</p>
          </div>
          <div>
            <label for="tp-type" class="field-label">{{ $t('tariffs.editor.type') }}</label>
            <select id="tp-type" v-model="form.planType" class="field">
              <option value="BANDS">{{ $t('tariffs.editor.typeBands') }}</option>
              <option value="FLAT">{{ $t('tariffs.editor.typeFlat') }}</option>
            </select>
          </div>
        </div>

        <div v-if="form.planType === 'FLAT'" class="max-w-xs">
          <label for="tp-flat" class="field-label">{{ $t('tariffs.editor.pricePerKwh', { cur: symbol }) }}</label>
          <NumberInput id="tp-flat" v-model="form.flatRate" min="0" class="field" />
          <p class="mt-1 text-xs text-slate-400">{{ $t('tariffs.editor.roundingHint') }}</p>
        </div>

        <template v-else>
          <section class="space-y-2" aria-labelledby="tp-bands">
            <h4 id="tp-bands" class="text-sm font-semibold text-slate-200">{{ $t('tariffs.editor.bands') }}</h4>
            <p class="text-xs text-slate-400">{{ $t('tariffs.editor.bandsHint') }} {{ $t('tariffs.editor.roundingHint') }}</p>
            <div v-for="(band, i) in form.bands" :key="i" class="flex items-end gap-2">
              <div class="flex-1 min-w-0">
                <label :for="`tp-band-name-${i}`" class="field-label">{{ $t('tariffs.editor.bandName') }}</label>
                <input :id="`tp-band-name-${i}`" :value="band.name" maxlength="50" class="field" :placeholder="$t('tariffs.editor.bandNamePlaceholder')" @input="renameBand(form, i, ($event.target as HTMLInputElement).value)" />
              </div>
              <div class="w-32 shrink-0">
                <label :for="`tp-band-rate-${i}`" class="field-label">{{ $t('tariffs.editor.pricePerKwh', { cur: symbol }) }}</label>
                <NumberInput :id="`tp-band-rate-${i}`" v-model="band.rate" min="0" class="field" />
              </div>
              <button type="button" class="tap p-2 rounded-lg text-slate-400 hover:text-danger-400 hover:bg-danger-500/10 disabled:opacity-40" :disabled="form.bands.length < 2" :aria-label="$t('tariffs.editor.removeBand')" @click="removeBand(form, i)">
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
            <button type="button" class="btn btn-secondary" @click="addBand"><Plus class="w-3.5 h-3.5" />{{ $t('tariffs.editor.addBand') }}</button>
          </section>

          <section class="space-y-2" aria-labelledby="tp-rules">
            <h4 id="tp-rules" class="text-sm font-semibold text-slate-200">{{ $t('tariffs.editor.rules') }}</h4>
            <p class="text-xs text-slate-400">{{ $t('tariffs.editor.rulesHint') }}</p>
            <div class="max-w-xs">
              <label for="tp-default" class="field-label">{{ $t('tariffs.editor.defaultBand') }}</label>
              <select id="tp-default" v-model="form.defaultBand" class="field">
                <option value="">{{ namedBands[0] ?? '' }}</option>
                <option v-for="n in namedBands.slice(1)" :key="n" :value="n">{{ n }}</option>
              </select>
            </div>
            <div v-for="(rule, i) in form.rules" :key="i" class="rounded-xl border border-slate-800 bg-slate-950/60 p-3 space-y-2">
              <div class="flex flex-wrap items-end gap-2">
                <div>
                  <label :for="`tp-rule-start-${i}`" class="field-label">{{ $t('tariffs.editor.from') }}</label>
                  <input :id="`tp-rule-start-${i}`" v-model="rule.start" type="time" class="field" />
                </div>
                <div>
                  <label :for="`tp-rule-end-${i}`" class="field-label">{{ $t('tariffs.editor.to') }}</label>
                  <input :id="`tp-rule-end-${i}`" v-model="rule.end" type="time" class="field" />
                </div>
                <div class="flex-1 min-w-32">
                  <label :for="`tp-rule-band-${i}`" class="field-label">{{ $t('tariffs.editor.band') }}</label>
                  <select :id="`tp-rule-band-${i}`" v-model="rule.band" class="field">
                    <option v-for="n in namedBands" :key="n" :value="n">{{ n }}</option>
                  </select>
                </div>
                <button type="button" class="tap p-2 rounded-lg text-slate-400 hover:text-danger-400 hover:bg-danger-500/10" :aria-label="$t('tariffs.editor.removeRule')" @click="form.rules.splice(i, 1)">
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>
              <div role="group" :aria-label="$t('tariffs.editor.days')" class="flex flex-wrap gap-1.5">
                <button
                  v-for="d in WEEK_ORDER"
                  :key="d"
                  type="button"
                  :aria-pressed="rule.days.includes(d)"
                  class="px-2.5 py-1 rounded-lg border text-xs font-medium"
                  :class="rule.days.includes(d) ? 'border-primary-500 bg-primary-500/20 text-primary-200' : 'border-slate-700 text-slate-400 hover:text-white'"
                  @click="toggleDay(rule, d)"
                >
                  {{ dayLabel(d) }}
                </button>
                <span v-if="!rule.days.length" class="self-center text-xs text-slate-400">{{ $t('tariffs.editor.everyDay') }}</span>
              </div>
            </div>
            <button type="button" class="btn btn-secondary" :disabled="!namedBands.length" @click="addRule"><Plus class="w-3.5 h-3.5" />{{ $t('tariffs.editor.addRule') }}</button>
          </section>
        </template>

        <section class="grid sm:grid-cols-2 gap-3" aria-labelledby="tp-more">
          <h4 id="tp-more" class="sm:col-span-2 text-sm font-semibold text-slate-200">{{ $t('tariffs.editor.validity') }}</h4>
          <p class="sm:col-span-2 -mt-2 text-xs text-slate-400">{{ $t('tariffs.editor.validityHint') }}</p>
          <div>
            <label for="tp-from" class="field-label">{{ $t('tariffs.editor.validFrom') }}</label>
            <input id="tp-from" v-model="form.validFrom" type="date" class="field" />
          </div>
          <div>
            <label for="tp-to" class="field-label">{{ $t('tariffs.editor.validTo') }}</label>
            <input id="tp-to" v-model="form.validTo" type="date" class="field" />
          </div>
          <div>
            <label for="tp-standing" class="field-label">{{ $t('tariffs.editor.standingCharge', { cur: symbol }) }}</label>
            <NumberInput id="tp-standing" v-model="form.standingCharge" min="0" class="field" />
            <p class="mt-1 text-xs text-slate-400">{{ $t('tariffs.editor.standingChargeHint') }}</p>
          </div>
          <label class="flex items-start gap-2 text-sm text-slate-200 sm:self-center">
            <input v-model="form.isDefault" type="checkbox" class="mt-0.5" />
            <span>{{ $t('tariffs.editor.isDefault') }}</span>
          </label>
        </section>

        <p v-if="error" role="alert" class="rounded-lg border border-danger-500/30 bg-danger-500/10 px-3 py-2 text-xs text-danger-300">{{ error }}</p>
      </div>

      <div class="px-5 py-3 border-t border-slate-800 flex items-center justify-between gap-3 shrink-0">
        <p class="text-xs text-warning-300 min-w-0" role="status">{{ problemText }}</p>
        <div class="flex gap-2 shrink-0">
          <button type="button" class="btn btn-secondary" @click="open = false">{{ $t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving || !!problem">{{ saving ? $t('account.saving') : $t('common.save') }}</button>
        </div>
      </div>
    </form>
  </div>
</template>
