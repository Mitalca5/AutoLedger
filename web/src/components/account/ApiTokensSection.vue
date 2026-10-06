<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Key, Plus, Trash2, Copy, Check, ExternalLink, ShieldAlert, Sparkles } from 'lucide-vue-next'
import { api, type APITokenInfo, type APITokenCreatedResponse } from '@/services/api'
import { describeRelativeTime } from '@/utils/userAgent'
import { useConfirm } from '@/composables/useConfirm'
import { t } from '@/i18n'
import LoadError from '@/components/LoadError.vue'

const { showConfirm } = useConfirm()

const tokens = ref<APITokenInfo[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)
const creating = ref(false)
const showCreateModal = ref(false)
const tokenName = ref('')
const tokenExpiresDays = ref<number | null>(null)
const createError = ref('')
const createdTokenResponse = ref<APITokenCreatedResponse | null>(null)
const copied = ref(false)

async function loadTokens() {
  loading.value = true
  loadError.value = null
  try {
    const res = await api.getAPITokens()
    tokens.value = res.tokens || []
  } catch (err) {
    console.error('Failed to load API tokens', err)
    loadError.value = (err as Error)?.message ?? ''
  } finally {
    loading.value = false
  }
}

async function handleCreateToken() {
  if (!tokenName.value.trim()) return
  creating.value = true
  createError.value = ''
  try {
    let expiresAt: string | null = null
    if (tokenExpiresDays.value && tokenExpiresDays.value > 0) {
      const d = new Date()
      d.setDate(d.getDate() + tokenExpiresDays.value)
      expiresAt = d.toISOString()
    }
    const res = await api.createAPIToken({
      name: tokenName.value.trim(),
      expires_at: expiresAt,
    })
    createdTokenResponse.value = res
    tokenName.value = ''
    tokenExpiresDays.value = null
    await loadTokens()
  } catch (err: any) {
    createError.value = err?.message || t('account.tokens.createFailed')
  } finally {
    creating.value = false
  }
}

async function copyToken() {
  if (!createdTokenResponse.value?.token) return
  await navigator.clipboard.writeText(createdTokenResponse.value.token)
  copied.value = true
  setTimeout(() => {
    copied.value = false
  }, 2000)
}

function closeCreateModal() {
  showCreateModal.value = false
  createdTokenResponse.value = null
  createError.value = ''
}

async function revokeToken(token: APITokenInfo) {
  const ok = await showConfirm({
    title: t('account.tokens.revokeTitle'),
    message: t('account.tokens.revokeMessage', { name: token.name }),
    confirmText: t('account.tokens.revoke'),
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.revokeAPIToken(token.id)
    tokens.value = tokens.value.filter((t) => t.id !== token.id)
  } catch (err) {
    console.error('Failed to revoke API token', err)
  }
}

onMounted(() => {
  loadTokens()
})
</script>

<template>
  <section class="rounded-2xl border border-slate-800 bg-slate-900 p-5 space-y-4" aria-labelledby="account-api-tokens">
    <div class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 pb-3">
      <div>
        <h2 id="account-api-tokens" class="flex items-center gap-2 text-sm font-bold text-white">
          <Key class="h-4 w-4 text-rose-400" aria-hidden="true" />
          {{ t('account.tokens.title') }}
        </h2>
        <p class="text-xs text-slate-400 mt-0.5">
          {{ t('account.tokens.subtitle') }}
        </p>
      </div>
      <button
        type="button"
        @click="showCreateModal = true"
        class="btn btn-primary"
      >
        <Plus class="h-3.5 w-3.5" />
        {{ t('account.tokens.create') }}
      </button>
    </div>

    <!-- Home Assistant integration Info Banner -->
    <div class="rounded-xl border border-blue-500/20 bg-blue-500/10 p-3.5 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div class="flex items-start gap-2.5">
        <Sparkles class="h-4 w-4 text-blue-400 shrink-0 mt-0.5" />
        <div>
          <h3 class="text-xs font-semibold text-blue-200">{{ t('account.tokens.haIntegrationTitle') }}</h3>
          <p class="text-xs text-blue-300/80 mt-0.5">
            {{ t('account.tokens.haIntegrationSubtitle') }}
          </p>
        </div>
      </div>
      <a
        href="https://github.com/Rem7474/autoledger-homeassistant"
        target="_blank"
        rel="noopener noreferrer"
        class="tap-text justify-center gap-1.5 px-3 rounded-lg bg-blue-600/30 hover:bg-blue-600/50 border border-blue-500/40 text-blue-200 text-xs font-medium transition-colors shrink-0"
      >
        <ExternalLink class="h-3 w-3" />
        {{ t('account.tokens.haIntegrationLink') }}
      </a>
    </div>

    <!-- Tokens List -->
    <div v-if="loading" class="text-sm text-slate-400 py-2">
      {{ t('account.tokens.loading') }}
    </div>
    <LoadError v-else-if="loadError !== null" :message="loadError" @retry="loadTokens" />
    <div v-else-if="tokens.length === 0" class="text-center py-6 text-xs text-slate-400">
      {{ t('account.tokens.noTokens') }}
    </div>
    <ul v-else class="space-y-2">
      <li
        v-for="tok in tokens"
        :key="tok.id"
        class="flex items-center justify-between gap-3 rounded-xl border border-slate-800 bg-slate-950/60 p-3"
      >
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <span class="text-sm font-semibold text-white truncate">{{ tok.name }}</span>
            <span class="font-mono text-xs px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
              {{ tok.token_prefix }}...
            </span>
          </div>
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-400 mt-1">
            <span>{{ t('account.tokens.created', { date: describeRelativeTime(tok.created_at) }) }}</span>
            <span v-if="tok.last_used_at">
              {{ t('account.tokens.lastUsed', { date: describeRelativeTime(tok.last_used_at) }) }}
            </span>
            <span v-else class="text-slate-400">{{ t('account.tokens.neverUsed') }}</span>
            <span v-if="tok.expires_at" class="text-warning-400/80">
              {{ t('account.tokens.expires', { date: describeRelativeTime(tok.expires_at) }) }}
            </span>
          </div>
        </div>

        <button
          type="button"
          @click="revokeToken(tok)"
          class="p-2 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors"
          :title="t('account.tokens.revoke')" :aria-label="t('account.tokens.revoke')"
        >
          <Trash2 class="h-4 w-4" />
        </button>
      </li>
    </ul>

    <!-- Modal Create Token -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
      @click.self="closeCreateModal"
    >
      <div v-dialog="closeCreateModal" class="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-xl space-y-4">
        <!-- Step 1: Input -->
        <template v-if="!createdTokenResponse">
          <h3 class="text-base font-bold text-white">{{ t('account.tokens.modalTitle') }}</h3>
          <p class="text-xs text-slate-400">{{ t('account.tokens.modalSubtitle') }}</p>

          <form @submit.prevent="handleCreateToken" class="space-y-3">
            <div>
              <label for="token-name-input" class="block text-xs font-medium text-slate-300 mb-1">
                {{ t('account.tokens.nameLabel') }}
              </label>
              <input
                id="token-name-input"
                v-model="tokenName"
                type="text"
                required
                maxlength="100"
                :placeholder="t('account.tokens.namePlaceholder')"
                class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-white placeholder-slate-500 focus:border-rose-500 focus:outline-none"
              />
            </div>

            <div>
              <label for="token-expiry-select" class="block text-xs font-medium text-slate-300 mb-1">
                {{ t('account.tokens.expiryLabel') }}
              </label>
              <select
                id="token-expiry-select"
                v-model="tokenExpiresDays"
                class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-white focus:border-rose-500 focus:outline-none"
              >
                <option :value="null">{{ t('account.tokens.noExpiry') }}</option>
                <option :value="30">30 {{ t('account.tokens.days') }}</option>
                <option :value="90">90 {{ t('account.tokens.days') }}</option>
                <option :value="365">1 {{ t('account.tokens.year') }}</option>
              </select>
            </div>

            <p v-if="createError" class="text-xs text-danger-400 bg-danger-500/10 border border-danger-500/20 rounded-lg p-2">
              {{ createError }}
            </p>

            <div class="flex items-center justify-end gap-2 pt-2">
              <button
                type="button"
                @click="closeCreateModal"
                class="btn btn-lg btn-secondary"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="submit"
                :disabled="creating || !tokenName.trim()"
                class="btn btn-lg btn-primary"
              >
                {{ creating ? t('account.tokens.generating') : t('account.tokens.generate') }}
              </button>
            </div>
          </form>
        </template>

        <!-- Step 2: Display Token (One time) -->
        <template v-else>
          <div class="space-y-3">
            <h3 class="text-base font-bold text-white">{{ t('account.tokens.tokenGeneratedTitle') }}</h3>
            <div class="p-3 bg-warning-500/10 border border-warning-500/20 rounded-xl flex items-start gap-2.5">
              <ShieldAlert class="h-4 w-4 text-warning-400 shrink-0 mt-0.5" />
              <p class="text-xs text-warning-200">
                {{ t('account.tokens.tokenWarning') }}
              </p>
            </div>

            <div class="relative">
              <div class="p-3 bg-slate-950 border border-slate-700 rounded-xl font-mono text-xs text-success-400 break-all select-all pr-12">
                {{ createdTokenResponse.token }}
              </div>
              <button
                type="button"
                @click="copyToken"
                class="absolute right-2 top-2 p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                :title="t('account.tokens.copy')" :aria-label="t('account.tokens.copy')"
              >
                <Check v-if="copied" class="h-4 w-4 text-success-400" />
                <Copy v-else class="h-4 w-4" />
              </button>
            </div>

            <div class="flex justify-end pt-2">
              <button
                type="button"
                @click="closeCreateModal"
                class="btn btn-lg btn-primary"
              >
                {{ t('common.done') }}
              </button>
            </div>
          </div>
        </template>
      </div>
    </div>
  </section>
</template>
