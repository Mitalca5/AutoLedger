<script setup lang="ts">
import { t } from '@/i18n'
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useVehicleStore } from '@/stores/vehicle'
import { Zap, Lock, Mail, AlertCircle, Shield } from 'lucide-vue-next'
import { APP_NAME } from '@/brand'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import { APP_VERSION } from '@/version'
import { ssoLoginErrorKey } from '@/utils/sso'

const router = useRouter()
const authStore = useAuthStore()

const email = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const registrationEnabled = ref(true)
const oidcEnabled = ref(false)
const oidcProviderName = ref('SSO')
const demoAccount = ref<{ email: string; password: string } | null>(null)

onMounted(async () => {
  try {
    const res = await fetch('/api/auth/config')
    if (res.ok) {
      const data = await res.json()
      registrationEnabled.value = data.registration_enabled
      oidcEnabled.value = !!data.oidc_enabled
      oidcProviderName.value = data.oidc_provider_name || 'SSO'
      if (data.demo && data.demo_email && data.demo_password) {
        demoAccount.value = { email: data.demo_email, password: data.demo_password }
      }
    }
  } catch {
    // defaults
  }

  // Show OIDC error if redirected back after a failure
  const errorKey = ssoLoginErrorKey(new URLSearchParams(window.location.search).get('error'))
  if (errorKey) error.value = t(errorKey)
})

async function handleSubmit() {
  await signIn(email.value, password.value)
}

async function openDemo() {
  if (demoAccount.value) await signIn(demoAccount.value.email, demoAccount.value.password)
}

async function signIn(emailValue: string, passwordValue: string) {
  error.value = ''
  loading.value = true
  try {
    await authStore.login({ email: emailValue, password: passwordValue })
    const vehicleStore = useVehicleStore()
    await vehicleStore.fetchVehicles()
    router.push('/')
  } catch (err: any) {
    error.value = err.message || t('auth.loginView.invalidCredentials')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-4 bg-slate-950">
    <div class="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl p-6 sm:p-8 shadow-2xl">
      <div class="text-center mb-8">
        <div class="inline-flex p-3 bg-gradient-to-tr from-rose-500 to-warning-500 rounded-2xl shadow-lg shadow-rose-500/20 mb-3">
          <Zap class="w-8 h-8 text-white" />
        </div>
        <h1 class="text-2xl font-bold tracking-tight text-white">{{ $t('auth.loginView.signInTo', { APP_NAME }) }}</h1>
        <p class="text-sm text-slate-400 mt-1">{{ $t('auth.loginView.totalCostOfOwnershipTco') }}</p>
      </div>

      <div v-if="error" class="mb-4 p-3 bg-danger-500/10 border border-danger-500/20 rounded-xl flex items-center gap-2 text-sm text-danger-400">
        <AlertCircle class="w-4 h-4 shrink-0" />
        <span>{{ error }}</span>
      </div>

      <button
        v-if="demoAccount"
        type="button"
        :disabled="loading"
        class="w-full mb-4 py-3 px-4 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white font-semibold rounded-xl shadow-lg shadow-rose-600/25 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
        @click="openDemo"
      >
        {{ loading ? $t('auth.loginView.openingTheDemo') : $t('auth.loginView.tryTheDemo') }}
      </button>

      <!-- OIDC / SSO login button (shown only when OIDC is configured server-side) -->
      <a
        v-if="oidcEnabled"
        href="/api/auth/oidc/login"
        class="w-full flex items-center justify-center gap-2 py-3 px-4 border border-slate-600 rounded-xl text-white hover:bg-slate-800 transition-all font-medium text-sm mb-4"
      >
        <Shield class="w-4 h-4 text-rose-400" />
        {{ $t('auth.loginView.signInWith', { oidcProviderName }) }}
      </a>

      <!-- Separator: shown only when OIDC is enabled AND local form is still visible -->
      <div v-if="oidcEnabled" class="relative my-4 flex items-center">
        <div class="flex-grow border-t border-slate-700" />
        <span class="mx-3 text-xs text-slate-400">{{ $t('common.or') }}</span>
        <div class="flex-grow border-t border-slate-700" />
      </div>

      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div>
          <label for="login-email" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('auth.loginView.email') }}</label>
          <div class="relative">
            <Mail class="w-5 h-5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input id="login-email"
              v-model="email"
              type="email"
              required
              :placeholder="$t('auth.loginView.youExampleCom')"
              class="field pl-10 pr-4 placeholder-slate-500"
            />
          </div>
        </div>

        <div>
          <label for="login-password" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('auth.loginView.password') }}</label>
          <div class="relative">
            <Lock class="w-5 h-5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input id="login-password"
              v-model="password"
              type="password"
              required
              placeholder="••••••••"
              class="field pl-10 pr-4 placeholder-slate-500"
            />
          </div>
        </div>

        <button
          type="submit"
          :disabled="loading"
          class="w-full py-3 px-4 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white font-semibold rounded-xl shadow-lg shadow-rose-600/25 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {{ loading ? $t('auth.loginView.signingIn') : $t('auth.loginView.signIn') }}
        </button>
      </form>

      <div v-if="registrationEnabled" class="mt-6 text-center text-sm text-slate-400">
        {{ $t('auth.loginView.noAccountYet') }}
        <router-link to="/register" class="text-rose-400 hover:text-rose-300 font-medium">{{ $t('auth.loginView.createAnAccount') }}</router-link>
      </div>

      <div class="mt-6 pt-4 border-t border-slate-800 flex flex-col items-center gap-2">
        <LanguageSwitcher />
        <span class="text-xs font-mono text-slate-400">{{ APP_NAME }} {{ APP_VERSION }}</span>
      </div>
    </div>
  </div>
</template>
