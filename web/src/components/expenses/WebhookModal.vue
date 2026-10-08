<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import { t } from '@/i18n'
import { computed, ref, watch } from 'vue'
import { api, type VehicleWebhook } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { AlertTriangle, Loader2, CheckCircle2, Radio } from 'lucide-vue-next'

// The vehicle's notification webhook. `webhook` is the saved one (null when none); changes are reported through update:webhook.
const props = defineProps<{ vehicleId: string; webhook: VehicleWebhook | null }>()
const emit = defineEmits<{ 'update:webhook': [webhook: VehicleWebhook | null] }>()
const open = defineModel<boolean>('open', { required: true })
const { showConfirm, showAlert } = useConfirm()

const vehicleWebhook = computed(() => props.webhook)
const webhookForm = ref({
  url: '',
  type: 'DISCORD',
  enabled: true,
})
const isTestingWebhook = ref(false)
const isSavingWebhook = ref(false)
const webhookTestResult = ref<{ success: boolean; message: string } | null>(null)

watch(open, (isOpen) => {
  if (!isOpen) return
  webhookTestResult.value = null
  const wh = props.webhook
  webhookForm.value = wh ? { url: wh.url, type: wh.type, enabled: wh.enabled } : { url: '', type: 'DISCORD', enabled: true }
})

async function handleSaveWebhook() {
  if (!props.vehicleId) return
  if (!webhookForm.value.url.trim()) {
    showAlert(t('expenses.webhookModal.enterValidUrl'), t('common.requiredField'), 'warning')
    return
  }
  isSavingWebhook.value = true
  try {
    const saved = await api.saveVehicleWebhook(props.vehicleId, {
      url: webhookForm.value.url.trim(),
      type: webhookForm.value.type,
      enabled: webhookForm.value.enabled,
    })
    emit('update:webhook', saved)
    showAlert(t('expenses.webhookModal.saved'), t('common.success'), 'success')
    open.value = false
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    isSavingWebhook.value = false
  }
}

async function handleTestWebhook() {
  if (!props.vehicleId) return
  if (!webhookForm.value.url.trim()) {
    showAlert(t('expenses.webhookModal.enterUrlFirst'), t('common.requiredField'), 'warning')
    return
  }
  isTestingWebhook.value = true
  webhookTestResult.value = null
  try {
    const res = await api.testVehicleWebhook(props.vehicleId, {
      url: webhookForm.value.url.trim(),
      type: webhookForm.value.type,
    })
    webhookTestResult.value = res
  } catch (err: any) {
    webhookTestResult.value = { success: false, message: err.message || t('expenses.webhookModal.unexpectedError') }
  } finally {
    isTestingWebhook.value = false
  }
}

async function handleDeleteWebhook() {
  if (!props.vehicleId) return
  const ok = await showConfirm({
    title: t('expenses.webhookModal.deleteTitle'),
    message: t('expenses.webhookModal.deleteMessage'),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteVehicleWebhook(props.vehicleId)
    emit('update:webhook', null)
    webhookForm.value = { url: '', type: 'DISCORD', enabled: true }
    showAlert(t('expenses.webhookModal.deleted'), t('common.success'), 'success')
    open.value = false
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <ModalShell
    v-model:open="open"
    :title="$t('expenses.webhookModal.homelabWebhookNotifications')"
    :icon="Radio"
    icon-class="text-violet-400"
    footer-class="items-center justify-between"
  >
    <form id="webhook-modal-form" @submit.prevent="handleSaveWebhook" class="space-y-4">
      <p class="text-xs text-slate-400">
        {{ $t('expenses.webhookModal.setUpAnOutgoingWebhook') }}
      </p>

      <div>
        <label for="webhook-form-type" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.webhookModal.platformConnector') }}</label>
        <select
          id="webhook-form-type"
          v-model="webhookForm.type"
          class="field"
        >
          <option value="DISCORD">{{ $t('expenses.webhookModal.discordWebhookEmbed') }}</option>
          <option value="TELEGRAM">{{ $t('expenses.webhookModal.telegramBotSendmessageApi') }}</option>
          <option value="GOTIFY">{{ $t('expenses.webhookModal.gotifyPushNotification') }}</option>
          <option value="GENERIC">{{ $t('expenses.webhookModal.genericStandardJson') }}</option>
        </select>
      </div>

      <div>
        <label for="webhook-form-url" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.webhookModal.webhookTargetUrl') }}</label>
        <input
          id="webhook-form-url"
          v-model="webhookForm.url"
          type="url"
          required
          :placeholder="$t('expenses.webhookModal.httpsDiscordComApiWebhooks')"
          class="field"
        />
        <p class="text-xs text-slate-400 mt-1">
          {{ $t('expenses.webhookModal.forTelegramTheUrlMust') }}
        </p>
      </div>

      <div class="flex items-center gap-2 pt-1">
        <input
          id="webhook-form-enabled"
          v-model="webhookForm.enabled"
          type="checkbox"
          class="rounded border-slate-700 bg-slate-800 text-violet-600 focus:ring-violet-500"
        />
        <label for="webhook-form-enabled" class="text-xs text-slate-300 cursor-pointer">
          {{ $t('expenses.webhookModal.enableAutomaticBackgroundNotifications') }}
        </label>
      </div>

      <!-- Test result banner -->
      <div
        v-if="webhookTestResult"
        class="p-3 rounded-xl border text-xs flex items-center gap-2"
        :class="webhookTestResult.success ? 'bg-success-500/10 border-success-500/30 text-success-300' : 'bg-rose-500/10 border-rose-500/30 text-rose-300'"
      >
        <CheckCircle2 v-if="webhookTestResult.success" class="w-4 h-4 shrink-0 text-success-400" />
        <AlertTriangle v-else class="w-4 h-4 shrink-0 text-danger-400" />
        <span>{{ webhookTestResult.message }}</span>
      </div>

      <!-- Test button -->
      <div class="pt-2">
        <button
          type="button"
          @click="handleTestWebhook"
          :disabled="isTestingWebhook || !webhookForm.url"
          class="btn btn-lg btn-secondary w-full"
        >
          <Loader2 v-if="isTestingWebhook" class="w-4 h-4 animate-spin text-violet-400" />
          <Radio v-else class="w-4 h-4 text-violet-400" />
          <span>{{ isTestingWebhook ? $t('expenses.webhookModal.sendingTest') : $t('expenses.webhookModal.sendTest') }}</span>
        </button>
      </div>
    </form>
    <template #footer>
      <div>
        <button
          v-if="vehicleWebhook"
          type="button"
          @click="handleDeleteWebhook"
          class="px-3 py-1.5 bg-rose-900/20 hover:bg-rose-900/40 text-rose-400 text-xs font-semibold rounded-xl border border-rose-800/40 transition-colors"
        >
          {{ $t('expenses.webhookModal.deleteTheWebhook') }}
        </button>
      </div>
      <div class="flex items-center gap-2">
        <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
          {{ $t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="webhook-modal-form"
          :disabled="isSavingWebhook"
          class="btn btn-lg btn-primary"
        >
          <Loader2 v-if="isSavingWebhook" class="w-3.5 h-3.5 animate-spin" />
          <span>{{ $t('common.save') }}</span>
        </button>
      </div>
    </template>
  </ModalShell>
</template>