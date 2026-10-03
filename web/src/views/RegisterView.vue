<script setup lang="ts">
import { t } from '@/i18n'
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useVehicleStore } from '@/stores/vehicle'
import { Zap, Lock, Mail, AlertCircle, ShieldAlert } from 'lucide-vue-next'
import { APP_NAME } from '@/brand'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import { APP_VERSION } from '@/version'

const router = useRouter()
const authStore = useAuthStore()
const vehicleStore = useVehicleStore()

const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const error = ref('')
const loading = ref(false)
const registrationEnabled = ref(true)

onMounted(async () => {
  try {
    const res = await fetch('/api/auth/config')
    if (res.ok) {
      const data = await res.json()
      registrationEnabled.value = data.registration_enabled
    }
  } catch {
    // default true
  }
})

async function handleSubmit() {
  error.value = ''
  if (password.value !== confirmPassword.value) {
    error.value = t('auth.registerView.passwordsDiffer')
    return
  }
  if (password.value.length < 8) {
    error.value = t('auth.registerView.passwordTooShort')
    return
  }

  loading.value = true
  try {
    await authStore.register({ email: email.value, password: password.value })
    // The app only loads the vehicles at startup when already signed in; without this the layout waits forever
    await vehicleStore.fetchVehicles()
    router.push('/vehicles')
  } catch (err: any) {
    error.value = err.message || t('auth.registerView.registrationFailed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-4 bg-slate-950">
    <div class="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl p-6 sm:p-8 shadow-2xl">
      <div class="text-center mb-8">
        <div class="inline-flex p-3 bg-gradient-to-tr from-rose-500 to-amber-500 rounded-2xl shadow-lg shadow-rose-500/20 mb-3">
          <Zap class="w-8 h-8 text-white" />
        </div>
        <h1 class="text-2xl font-bold tracking-tight text-white">{{ $t('auth.registerView.createAnAccount') }}</h1>
        <p class="text-sm text-slate-400 mt-1">{{ $t('auth.registerView.joinToManageYourVehicle', { APP_NAME }) }}</p>
      </div>

      <div v-if="error" class="mb-4 p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-center gap-2 text-sm text-rose-400">
        <AlertCircle class="w-4 h-4 shrink-0" />
        <span>{{ error }}</span>
      </div>

      <div v-if="!registrationEnabled" class="text-center py-4">
        <div class="p-4 bg-amber-500/10 border border-amber-500/20 rounded-xl flex flex-col items-center gap-2 text-amber-400 text-sm mb-6">
          <ShieldAlert class="w-8 h-8 text-amber-400" />
          <p class="font-medium">{{ $t('auth.registerView.registrationIsClosed') }}</p>
          <p class="text-xs text-amber-300/80">{{ $t('auth.registerView.theAdministratorHasClosedAccount') }}</p>
        </div>
        <router-link
          to="/login"
          class="w-full inline-block py-3 px-4 bg-slate-800 hover:bg-slate-700 text-white font-semibold rounded-xl transition-colors"
        >
          {{ $t('auth.registerView.backToTheSignIn') }}
        </router-link>
      </div>

      <form v-else @submit.prevent="handleSubmit" class="space-y-4">
        <div>
          <label for="register-email" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('auth.registerView.email') }}</label>
          <div class="relative">
            <Mail class="w-5 h-5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input id="register-email"
              v-model="email"
              type="email"
              required
              :placeholder="$t('auth.registerView.youExampleCom')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl pl-10 pr-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>
        </div>

        <div>
          <label for="register-password" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('auth.registerView.password8CharactersMin') }}</label>
          <div class="relative">
            <Lock class="w-5 h-5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input id="register-password"
              v-model="password"
              type="password"
              required
              placeholder="••••••••"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl pl-10 pr-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>
        </div>

        <div>
          <label for="register-confirm-password" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('auth.registerView.confirmThePassword') }}</label>
          <div class="relative">
            <Lock class="w-5 h-5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input id="register-confirm-password"
              v-model="confirmPassword"
              type="password"
              required
              placeholder="••••••••"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl pl-10 pr-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>
        </div>

        <button
          type="submit"
          :disabled="loading"
          class="w-full py-3 px-4 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white font-semibold rounded-xl shadow-lg shadow-rose-600/25 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {{ loading ? $t('auth.registerView.creating') : $t('auth.registerView.signUp') }}
        </button>
      </form>

      <div v-if="registrationEnabled" class="mt-6 text-center text-sm text-slate-400">
        {{ $t('auth.registerView.alreadyHaveAnAccount') }}
        <router-link to="/login" class="text-rose-400 hover:text-rose-300 font-medium">{{ $t('auth.registerView.signIn') }}</router-link>
      </div>

      <div class="mt-6 pt-4 border-t border-slate-800 flex flex-col items-center gap-2">
        <LanguageSwitcher />
        <span class="text-xs font-mono text-slate-400">{{ APP_NAME }} {{ APP_VERSION }}</span>
      </div>
    </div>
  </div>
</template>
