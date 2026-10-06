<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import { UserRound as PageIcon } from 'lucide-vue-next'
import { t } from '@/i18n'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import DistanceUnitSwitcher from '@/components/DistanceUnitSwitcher.vue'
import ApiTokensSection from '@/components/account/ApiTokensSection.vue'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { KeyRound, Laptop, LogOut, SlidersHorizontal, Smartphone, UserRound } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { usePreferencesStore } from '@/stores/preferences'
import { useConfirm } from '@/composables/useConfirm'
import { describeRelativeTime, describeUserAgent } from '@/utils/userAgent'

interface Session {
  id: string
  started_at: string
  last_used_at: string
  ip?: string
  user_agent?: string
  current: boolean
}

const router = useRouter()
const authStore = useAuthStore()
const prefs = usePreferencesStore()
const { showConfirm } = useConfirm()

const sessions = ref<Session[]>([])
const loading = ref(true)
const loadError = ref('')
const busyId = ref<string | null>(null)
const busyAll = ref(false)
const sessionError = ref('')

const hasPassword = computed(() => authStore.user?.has_password === true)

const current = ref('')
const next = ref('')
const confirmation = ref('')
const saving = ref(false)
const passwordError = ref('')
const passwordDone = ref('')

const isMobile = (ua?: string) => /Android|iPhone|iPad|iPod|Mobile/i.test(ua ?? '')

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    sessions.value = await api.getSessions()
  } catch (err: any) {
    loadError.value = err?.message || t('account.loadFailed')
  } finally {
    loading.value = false
  }
}

// The server has cleared the cookies when the current session is closed: only the local state is left to reset.
async function endLocalSession() {
  await authStore.logout()
  router.push('/login')
}

async function revoke(session: Session) {
  sessionError.value = ''
  if (!session.current) {
    const ok = await showConfirm({
      title: t('account.disconnectThisDevice'),
      message: t('account.disconnectDeviceMessage', { device: describeUserAgent(session.user_agent) }),
      confirmText: t('account.disconnect'),
      type: 'warning',
    })
    if (!ok) return
  }
  busyId.value = session.id
  try {
    await api.revokeSession(session.id)
    if (session.current) {
      await endLocalSession()
      return
    }
    sessions.value = sessions.value.filter((s) => s.id !== session.id)
  } catch (err: any) {
    sessionError.value = err?.message || t('account.disconnectFailed')
  } finally {
    busyId.value = null
  }
}

async function logoutEverywhere() {
  sessionError.value = ''
  const ok = await showConfirm({
    title: t('account.signOutAllTitle'),
    message: t('account.signOutAllMessage'),
    confirmText: t('account.signOutAll'),
    type: 'danger',
  })
  if (!ok) return
  busyAll.value = true
  try {
    await api.logoutAll()
    await endLocalSession()
  } catch (err: any) {
    sessionError.value = err?.message || t('account.disconnectAllFailed')
  } finally {
    busyAll.value = false
  }
}

async function changePassword() {
  passwordError.value = ''
  passwordDone.value = ''
  if (next.value.length < 8) {
    passwordError.value = t('account.passwordTooShort')
    return
  }
  if (new TextEncoder().encode(next.value).length > 72) {
    passwordError.value = t('account.passwordTooLong')
    return
  }
  if (next.value !== confirmation.value) {
    passwordError.value = t('account.passwordsDiffer')
    return
  }
  saving.value = true
  try {
    const res = await api.changePassword({ current_password: current.value, new_password: next.value })
    current.value = next.value = confirmation.value = ''
    const others = res?.sessions_revoked ?? 0
    passwordDone.value = others > 0
      ? t('account.passwordChangedOthers', { count: others })
      : t('account.passwordChanged')
    await load()
  } catch (err: any) {
    passwordError.value = err?.message || t('account.passwordChangeFailed')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-6">
    <PageHeader :title="$t('account.accountView.accountAndSecurity')" :icon="PageIcon">
      {{ $t('account.accountView.signInPasswordAndConnected') }}
    </PageHeader>

    <section class="rounded-2xl border border-slate-800 bg-slate-900 p-5" aria-labelledby="account-identity">
      <h2 id="account-identity" class="mb-3 flex items-center gap-2 text-sm font-bold text-white">
        <UserRound class="h-4 w-4 text-slate-400" aria-hidden="true" />
        {{ $t('account.accountView.identity') }}
      </h2>
      <dl class="space-y-1 text-sm">
        <div class="flex flex-wrap items-center gap-x-2">
          <dt class="text-slate-400">{{ $t('account.language') }}</dt>
          <dd><LanguageSwitcher /></dd>
        </div>
        <div class="flex flex-wrap gap-x-2">
          <dt class="text-slate-400">{{ $t('account.accountView.emailAddress') }}</dt>
          <dd class="break-all font-medium text-white">{{ authStore.user?.email }}</dd>
        </div>
        <div class="flex flex-wrap gap-x-2">
          <dt class="text-slate-400">{{ $t('account.accountView.signIn') }}</dt>
          <dd class="text-slate-200">{{ hasPassword ? $t('account.localPassword') : $t('account.ssoProvider') }}</dd>
        </div>
      </dl>
    </section>

    <section class="rounded-2xl border border-slate-800 bg-slate-900 p-5" aria-labelledby="account-preferences">
      <h2 id="account-preferences" class="mb-3 flex items-center gap-2 text-sm font-bold text-white">
        <SlidersHorizontal class="h-4 w-4 text-slate-400" aria-hidden="true" />
        {{ $t('account.accountView.preferences') }}
      </h2>
      <label class="flex cursor-pointer items-start justify-between gap-4">
        <span>
          <span class="block text-sm font-medium text-white">{{ $t('account.accountView.proPersoTitle') }}</span>
          <span class="block text-xs text-slate-400">{{ $t('account.accountView.proPersoHelp') }}</span>
        </span>
        <input
          type="checkbox"
          role="switch"
          class="mt-1 h-5 w-9 shrink-0 cursor-pointer appearance-none rounded-full bg-slate-700 transition-colors checked:bg-rose-500 relative before:absolute before:left-0.5 before:top-0.5 before:h-4 before:w-4 before:rounded-full before:bg-white before:transition-transform checked:before:translate-x-4"
          :checked="prefs.proPersoEnabled"
          @change="prefs.setProPersoEnabled(($event.target as HTMLInputElement).checked)"
        />
      </label>
      <div class="mt-4 flex flex-wrap items-center justify-between gap-4 border-t border-slate-800 pt-4">
        <span class="text-sm font-medium text-white">{{ $t('account.distanceUnit') }}</span>
        <DistanceUnitSwitcher />
      </div>
    </section>

    <section v-if="hasPassword" class="rounded-2xl border border-slate-800 bg-slate-900 p-5" aria-labelledby="account-password">
      <h2 id="account-password" class="mb-3 flex items-center gap-2 text-sm font-bold text-white">
        <KeyRound class="h-4 w-4 text-slate-400" aria-hidden="true" />
        {{ $t('account.accountView.password') }}
      </h2>
      <form class="space-y-4" novalidate @submit.prevent="changePassword">
        <div>
          <label for="pw-current" class="field-label">{{ $t('account.accountView.currentPassword') }}</label>
          <input id="pw-current" v-model="current" type="password" autocomplete="current-password" class="field" />
        </div>
        <div>
          <label for="pw-new" class="field-label">{{ $t('account.accountView.newPassword') }}</label>
          <input id="pw-new" v-model="next" type="password" autocomplete="new-password" class="field" aria-describedby="pw-hint" />
          <p id="pw-hint" class="mt-1 text-xs text-slate-400">{{ $t('account.accountView.8CharactersMinimum72Bytes') }}</p>
        </div>
        <div>
          <label for="pw-confirm" class="field-label">{{ $t('account.accountView.confirmTheNewPassword') }}</label>
          <input id="pw-confirm" v-model="confirmation" type="password" autocomplete="new-password" class="field" />
        </div>
        <p v-if="passwordError" role="alert" class="rounded-lg border border-danger-500/30 bg-danger-500/10 px-3 py-2 text-xs text-danger-300">{{ passwordError }}</p>
        <p v-if="passwordDone" role="status" class="rounded-lg border border-success-500/30 bg-success-500/10 px-3 py-2 text-xs text-success-300">{{ passwordDone }}</p>
        <button
          type="submit"
          :disabled="saving || !current || !next || !confirmation"
          class="btn btn-lg btn-primary min-h-12"
        >
          {{ saving ? $t('account.saving') : $t('account.changePassword') }}
        </button>
        <p class="text-xs text-slate-400">{{ $t('account.accountView.otherDevicesAreSignedOut') }}</p>
      </form>
    </section>

    <ApiTokensSection />

    <section class="rounded-2xl border border-slate-800 bg-slate-900 p-5" aria-labelledby="account-sessions">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 id="account-sessions" class="flex items-center gap-2 text-sm font-bold text-white">
          <Laptop class="h-4 w-4 text-slate-400" aria-hidden="true" />
          {{ $t('account.accountView.connectedDevices') }}
        </h2>
        <button
          type="button"
          :disabled="busyAll || loading"
          class="min-h-11 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 text-xs font-semibold text-rose-300 hover:bg-rose-500/20 disabled:opacity-50"
          @click="logoutEverywhere"
        >
          {{ $t('account.accountView.signOutEverywhere') }}
        </button>
      </div>

      <p v-if="loading" class="text-sm text-slate-400">{{ $t('account.accountView.loading') }}</p>
      <p v-else-if="loadError" role="alert" class="text-sm text-danger-300">{{ loadError }}</p>
      <ul v-else class="space-y-2">
        <li
          v-for="s in sessions"
          :key="s.id"
          class="flex items-center justify-between gap-3 rounded-xl border bg-slate-950/60 p-3"
          :class="s.current ? 'border-success-500/30' : 'border-slate-800'"
        >
          <div class="flex min-w-0 items-start gap-3">
            <component :is="isMobile(s.user_agent) ? Smartphone : Laptop" class="mt-0.5 h-5 w-5 shrink-0 text-slate-400" aria-hidden="true" />
            <div class="min-w-0">
              <p class="flex flex-wrap items-center gap-2 text-sm font-semibold text-white">
                {{ describeUserAgent(s.user_agent) }}
                <span v-if="s.current" class="rounded-full bg-success-500/15 px-2 py-0.5 text-xs font-bold uppercase tracking-wide text-success-300">{{ $t('account.accountView.thisDevice') }}</span>
              </p>
              <p class="text-xs text-slate-400">
                {{ $t('account.accountView.active', { value: describeRelativeTime(s.last_used_at) }) }}<template v-if="s.ip"> · {{ s.ip }}</template>
              </p>
              <p class="text-xs text-slate-400">{{ $t('account.accountView.signedIn', { value: describeRelativeTime(s.started_at) }) }}</p>
            </div>
          </div>
          <button
            type="button"
            :disabled="busyId === s.id"
            class="btn btn-secondary min-h-11 shrink-0"
            :aria-label="s.current ? $t('account.signOutThisDevice') : $t('account.signOutDevice', { device: describeUserAgent(s.user_agent) })"
            @click="revoke(s)"
          >
            <LogOut class="h-3.5 w-3.5" aria-hidden="true" />
            <span class="hidden sm:inline">{{ s.current ? $t('account.signOut') : $t('account.disconnect') }}</span>
          </button>
        </li>
        <li v-if="sessions.length === 0" class="text-sm text-slate-400">{{ $t('account.accountView.noActiveSession') }}</li>
      </ul>
      <p v-if="sessionError" role="alert" class="mt-3 text-xs text-danger-300">{{ sessionError }}</p>
      <p class="mt-3 text-xs text-slate-400">{{ $t('account.accountView.aSignedOutDeviceKeeps') }}</p>
    </section>
  </div>
</template>
