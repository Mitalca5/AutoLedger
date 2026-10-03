<script setup lang="ts">
import { currentLocale, intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { APP_NAME } from '@/brand'
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import { Zap, ShieldCheck, Car, KeyRound, ArrowRight, CheckCircle2, AlertCircle, RefreshCw, Link2, Webhook, FileSpreadsheet, Download, Copy } from 'lucide-vue-next'
import { distanceUnit, formatDistanceValue } from '@/units'
import { buildVehiclePayload, emptyTeslaMateForm, supportsTeslaMate, teslaMateCredentials, type Powertrain } from '@/utils/onboarding'
import { csvTemplate, csvTemplateFilename, csvTemplateTypes, webhookSnippet, type CsvTemplateType } from '@/utils/dataSources'
import { downloadCsv } from '@/utils/csv'

const router = useRouter()
const authStore = useAuthStore()
const vehicleStore = useVehicleStore()

const currentStep = ref(1)
const loading = ref(false)
const error = ref('')

// Step 1: Admin Account
const adminEmail = ref('')
const adminPassword = ref('')
const adminConfirmPassword = ref('')

// Step 2: Vehicle Setup
const vehicleMake = ref('')
const vehicleModel = ref('')
const vehicleName = ref('')
const vehicleVin = ref('')
const vehiclePowertrain = ref<Powertrain>('EV')
const vehicleOdometer = ref(15000)

function updateVehicleNameDefault() {
  const parts = [vehicleMake.value.trim(), vehicleModel.value.trim()].filter(Boolean)
  if (!vehicleName.value || vehicleName.value === parts.slice(0, -1).join(' ')) {
    vehicleName.value = parts.join(' ')
  }
}

// Step 3: data sources (optional, nothing preselected)
const teslamate = reactive(emptyTeslaMateForm())
const testResult = ref<{ ok: boolean; message: string } | null>(null)
const useWebhook = ref(false)
const webhook = ref<{ token: string; snippet: string } | null>(null)
const webhookFailed = ref(false)
const copiedField = ref<'token' | 'snippet' | ''>('')

function downloadTemplate(type: CsvTemplateType) {
  const { headers, rows } = csvTemplate(type, currentLocale())
  downloadCsv(csvTemplateFilename(type), headers, rows)
}

async function copy(field: 'token' | 'snippet') {
  if (!webhook.value) return
  try {
    await navigator.clipboard.writeText(webhook.value[field])
    copiedField.value = field
    setTimeout(() => (copiedField.value = ''), 2000)
  } catch {
    copiedField.value = ''
  }
}

onMounted(async () => {
  if (authStore.isAuthenticated) {
    await vehicleStore.fetchVehicles()
    if (vehicleStore.vehicles.length > 0) {
      router.push('/')
      return
    }
    // Admin account already exists & logged in, proceed to vehicle setup
    currentStep.value = 2
  }
})

async function handleStep1Submit() {
  error.value = ''
  if (adminPassword.value !== adminConfirmPassword.value) {
    error.value = t('onboarding.passwordsDiffer')
    return
  }
  if (adminPassword.value.length < 8) {
    error.value = t('onboarding.passwordTooShort')
    return
  }

  loading.value = true
  try {
    await authStore.register({ email: adminEmail.value, password: adminPassword.value })
    currentStep.value = 2
  } catch (err: any) {
    error.value = err.message || t('onboarding.adminCreationFailed')
  } finally {
    loading.value = false
  }
}

async function handleStep2Submit() {
  error.value = ''
  if (!vehicleName.value) {
    error.value = t('onboarding.vehicleNameRequired')
    return
  }
  currentStep.value = 3
}

async function testConnection() {
  testResult.value = null
  loading.value = true
  try {
    if (!teslamate.url) throw new Error(t('onboarding.apiUrlRequired'))
    const payload = {
      teslamate_api_url: teslamate.url,
      teslamate_auth_type: teslamate.authType,
      teslamate_car_id: 1,
      ...teslaMateCredentials(teslamate),
    }
    const res = await api.testTeslaMateRaw(payload)
    const st = res.status
    testResult.value = {
      ok: true,
      message: t('onboarding.testSuccess', { unit: distanceUnit(), state: st?.state || t('onboarding.online'), odometer: formatDistanceValue(st?.odometer || 0) }),
    }
  } catch (err: any) {
    testResult.value = { ok: false, message: err.message }
  } finally {
    loading.value = false
  }
}

async function handleFinalSubmit() {
  error.value = ''
  loading.value = true
  try {
    const payload = buildVehiclePayload({
      name: vehicleName.value,
      make: vehicleMake.value,
      model: vehicleModel.value,
      vin: vehicleVin.value,
      powertrain: vehiclePowertrain.value,
      odometer: vehicleOdometer.value,
      teslamate,
    })

    const vehicle = await api.createVehicle(payload)
    if (useWebhook.value) {
      try {
        const { token } = await api.createAPIToken({ name: 'Home Assistant' })
        webhook.value = { token, snippet: webhookSnippet(window.location.origin, token, vehicle.id) }
      } catch {
        webhookFailed.value = true
      }
    }
    await vehicleStore.fetchVehicles()
    currentStep.value = 4
  } catch (err: any) {
    error.value = err.message || t('onboarding.vehicleSaveFailed')
  } finally {
    loading.value = false
  }
}

function finishOnboarding() {
  router.push('/')
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-4 bg-slate-950">
    <div class="w-full max-w-xl bg-slate-900 border border-slate-800 rounded-3xl p-6 sm:p-10 shadow-2xl">
      <!-- Header -->
      <div class="text-center mb-8">
        <div class="inline-flex p-3.5 bg-gradient-to-tr from-rose-500 to-amber-500 rounded-2xl shadow-lg shadow-rose-500/25 mb-4">
          <Zap class="w-8 h-8 text-white" />
        </div>
        <h1 class="text-2xl sm:text-3xl font-bold tracking-tight text-white">{{ $t('onboarding.onboardingView.welcomeTo', { APP_NAME }) }}</h1>
        <p class="text-sm text-slate-400 mt-1.5">{{ $t('onboarding.onboardingView.initialSetupWizardForYour') }}</p>
      </div>

      <!-- Step Indicators -->
      <div class="flex items-center justify-between max-w-md mx-auto mb-8 relative">
        <div class="absolute left-0 top-1/2 -translate-y-1/2 h-0.5 w-full bg-slate-800 -z-0"></div>
        <div
          v-for="step in 3"
          :key="step"
          class="relative z-10 w-9 h-9 rounded-full flex items-center justify-center text-sm font-semibold transition-all"
          :class="[
            currentStep === step
              ? 'bg-rose-500 text-white shadow-lg shadow-rose-500/30 ring-4 ring-rose-500/20'
              : currentStep > step
              ? 'bg-emerald-500 text-white'
              : 'bg-slate-800 text-slate-400 border border-slate-700'
          ]"
        >
          <CheckCircle2 v-if="currentStep > step" class="w-5 h-5" />
          <span v-else>{{ step }}</span>
        </div>
      </div>

      <!-- Error alert -->
      <div v-if="error" class="mb-6 p-3.5 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-center gap-2.5 text-sm text-rose-400">
        <AlertCircle class="w-4 h-4 shrink-0" />
        <span>{{ error }}</span>
      </div>

      <!-- STEP 1: Admin Account Creation -->
      <div v-if="currentStep === 1">
        <div class="mb-6">
          <h2 class="text-lg font-semibold text-white flex items-center gap-2">
            <ShieldCheck class="w-5 h-5 text-rose-400" />
            {{ $t('onboarding.onboardingView.1CreateTheAdministratorAccount') }}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{{ $t('onboarding.onboardingView.thisIsTheMainAccount', { APP_NAME }) }}</p>
        </div>

        <form @submit.prevent="handleStep1Submit" class="space-y-4">
          <div>
            <label for="onboarding-admin-email" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.administratorEmail') }}</label>
            <input id="onboarding-admin-email"
              v-model="adminEmail"
              type="email"
              required
              :placeholder="$t('onboarding.onboardingView.adminYourDomainCom')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-3 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>

          <div>
            <label for="onboarding-admin-password" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.password8CharactersMin') }}</label>
            <input id="onboarding-admin-password"
              v-model="adminPassword"
              type="password"
              required
              placeholder=""
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-3 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>

          <div>
            <label for="onboarding-admin-confirm-password" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.confirmThePassword') }}</label>
            <input id="onboarding-admin-confirm-password"
              v-model="adminConfirmPassword"
              type="password"
              required
              placeholder=""
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-3 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>

          <button
            type="submit"
            :disabled="loading"
            class="w-full py-3.5 px-4 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white font-semibold rounded-xl shadow-lg shadow-rose-600/25 flex items-center justify-center gap-2 transition-all disabled:opacity-50"
          >
            <span>{{ loading ? $t('onboarding.creating') : $t('onboarding.continueToVehicle') }}</span>
            <ArrowRight class="w-4 h-4" />
          </button>
        </form>
      </div>

      <!-- STEP 2: Vehicle Setup -->
      <div v-else-if="currentStep === 2">
        <div class="mb-6">
          <h2 class="text-lg font-semibold text-white flex items-center gap-2">
            <Car class="w-5 h-5 text-rose-400" />
            {{ $t('onboarding.onboardingView.2YourFirstVehicle') }}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{{ $t('onboarding.onboardingView.setUpYourMainVehicle') }}</p>
        </div>

        <form @submit.prevent="handleStep2Submit" class="space-y-4">
          <!-- Make & Model Inputs -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label for="onboarding-vehicle-make" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.make') }}</label>
              <input id="onboarding-vehicle-make"
                v-model="vehicleMake"
                @input="updateVehicleNameDefault"
                type="text"
                :placeholder="$t('onboarding.onboardingView.makePlaceholder')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
              />
            </div>
            <div>
              <label for="onboarding-vehicle-model" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.model') }}</label>
              <input id="onboarding-vehicle-model"
                v-model="vehicleModel"
                @input="updateVehicleNameDefault"
                type="text"
                :placeholder="$t('onboarding.onboardingView.modelPlaceholder')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
              />
            </div>
          </div>

          <div>
            <label for="onboarding-vehicle-name" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.vehicleName') }}</label>
            <input id="onboarding-vehicle-name"
              v-model="vehicleName"
              type="text"
              required
              :placeholder="$t('onboarding.onboardingView.vehicleName')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
            />
          </div>

          <div>
            <label for="onboarding-vehicle-powertrain" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.powertrain') }}</label>
            <select id="onboarding-vehicle-powertrain"
              v-model="vehiclePowertrain"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white focus:outline-none focus:border-rose-500 transition-colors"
            >
              <option v-for="p in ['EV', 'PHEV', 'REEV', 'ICE']" :key="p" :value="p">{{ $t(`vehicles.powertrainOptions.${p}`) }}</option>
            </select>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
            <div>
              <label for="onboarding-vehicle-odometer" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.currentOdometerKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput id="onboarding-vehicle-odometer"
                v-model="vehicleOdometer"
                min="0"
                step="1"
                required
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
              />
            </div>
            <div>
              <label for="onboarding-vehicle-vin" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.vinOptional') }}</label>
              <input id="onboarding-vehicle-vin"
                v-model="vehicleVin"
                type="text"
                placeholder="VIN"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
              />
            </div>
          </div>

          <button
            type="submit"
            :disabled="loading"
            class="w-full py-3.5 px-4 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white font-semibold rounded-xl shadow-lg shadow-rose-600/25 flex items-center justify-center gap-2 transition-all disabled:opacity-50"
          >
            <span>{{ $t('onboarding.continueToSources') }}</span>
            <ArrowRight class="w-4 h-4" />
          </button>
        </form>
      </div>

      <!-- STEP 3: Data sources (optional) -->
      <div v-else-if="currentStep === 3">
        <div class="mb-6">
          <h2 class="text-lg font-semibold text-white flex items-center gap-2">
            <KeyRound class="w-5 h-5 text-rose-400" />
            {{ $t('onboarding.onboardingView.3TeslamateSynchronizationOptional') }}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{{ $t('onboarding.onboardingView.youCanConnectYourTeslamateapi') }}</p>
        </div>

        <div class="space-y-4">
          <label v-if="supportsTeslaMate(vehiclePowertrain)" class="flex items-center gap-3 p-4 bg-slate-800/60 border border-slate-700 rounded-xl cursor-pointer hover:bg-slate-800 transition-colors">
            <input v-model="teslamate.enabled" type="checkbox" class="w-5 h-5 shrink-0 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-900 border-slate-700" />
            <Link2 class="w-5 h-5 shrink-0 text-rose-400" />
            <div class="min-w-0">
              <span class="text-sm font-medium text-white block">{{ $t('onboarding.onboardingView.enableTheTeslamateapiLink') }}</span>
              <span class="text-xs text-slate-400 block">{{ $t('onboarding.onboardingView.automaticallySyncsDrivesChargesAnd') }}</span>
            </div>
          </label>

          <div v-if="supportsTeslaMate(vehiclePowertrain) && teslamate.enabled" class="p-4 bg-slate-800/40 border border-slate-800 rounded-2xl space-y-4">
            <div>
              <label for="onboarding-teslamate-url" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.teslamateApiUrl') }}</label>
              <input id="onboarding-teslamate-url"
                v-model="teslamate.url"
                type="url"
                :placeholder="$t('onboarding.onboardingView.http192168150')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
              />
            </div>

            <div>
              <label for="onboarding-teslamate-auth-type" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.authenticationMode') }}</label>
              <select id="onboarding-teslamate-auth-type"
                v-model="teslamate.authType"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white focus:outline-none focus:border-rose-500 transition-colors"
              >
                <option value="NONE">{{ $t('onboarding.onboardingView.noAuthentication') }}</option>
                <option value="BEARER">{{ $t('onboarding.onboardingView.apiKeyBearerToken') }}</option>
                <option value="BASIC">{{ $t('onboarding.onboardingView.httpBasicAuthUserPassword') }}</option>
              </select>
            </div>

            <div v-if="teslamate.authType === 'BEARER'">
              <label for="onboarding-teslamate-api-key" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.apiToken') }}</label>
              <input id="onboarding-teslamate-api-key"
                v-model="teslamate.apiKey"
                type="password"
                :placeholder="$t('onboarding.onboardingView.yourSecretToken')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
              />
            </div>

            <div v-if="teslamate.authType === 'BASIC'" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label for="onboarding-teslamate-user" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.user') }}</label>
                <input id="onboarding-teslamate-user"
                  v-model="teslamate.user"
                  type="text"
                  placeholder="admin"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
                />
              </div>
              <div>
                <label for="onboarding-teslamate-pass" class="block text-xs font-semibold text-slate-300 mb-1.5 uppercase tracking-wider">{{ $t('onboarding.onboardingView.password') }}</label>
                <input id="onboarding-teslamate-pass"
                  v-model="teslamate.pass"
                  type="password"
                  placeholder="••••••••"
                  class="w-full bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-rose-500 transition-colors"
                />
              </div>
            </div>

            <!-- Test Connection Button -->
            <div class="pt-2">
              <button
                type="button"
                @click="testConnection"
                :disabled="loading || !teslamate.url"
                class="w-full py-2.5 px-4 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center justify-center gap-2 border border-slate-700 disabled:opacity-50 transition-colors"
              >
                <RefreshCw v-if="loading" class="w-3.5 h-3.5 animate-spin text-rose-400" />
                <Link2 v-else class="w-3.5 h-3.5 text-rose-400" />
                <span>{{ loading ? $t('onboarding.testing') : $t('onboarding.testConnection') }}</span>
              </button>

              <div
                v-if="testResult"
                class="mt-2.5 p-3 rounded-xl text-xs flex items-start gap-2.5"
                :class="testResult.ok ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20'"
              >
                <CheckCircle2 v-if="testResult.ok" class="w-4 h-4 shrink-0 text-emerald-400 mt-0.5" />
                <AlertCircle v-else class="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
                <span>{{ testResult.message }}</span>
              </div>
            </div>
          </div>

          <label class="flex items-center gap-3 p-4 bg-slate-800/60 border border-slate-700 rounded-xl cursor-pointer hover:bg-slate-800 transition-colors">
            <input v-model="useWebhook" type="checkbox" class="w-5 h-5 shrink-0 rounded text-rose-500 focus:ring-rose-500/20 bg-slate-900 border-slate-700" />
            <Webhook class="w-5 h-5 shrink-0 text-rose-400" />
            <div class="min-w-0">
              <span class="text-sm font-medium text-white block">{{ $t('onboarding.webhookTitle') }}</span>
              <span class="text-xs text-slate-400 block">{{ $t('onboarding.webhookDescription') }}</span>
            </div>
          </label>

          <div class="p-4 bg-slate-800/60 border border-slate-700 rounded-xl">
            <div class="flex items-center gap-3">
              <span class="w-5 shrink-0" aria-hidden="true"></span>
              <FileSpreadsheet class="w-5 h-5 shrink-0 text-rose-400" />
              <div class="min-w-0">
                <span class="text-sm font-medium text-white block">{{ $t('onboarding.csvTitle') }}</span>
                <span class="text-xs text-slate-400 block">{{ $t('onboarding.csvDescription') }}</span>
              </div>
            </div>
            <div class="flex flex-wrap gap-2 mt-3">
              <button
                v-for="type in csvTemplateTypes(vehiclePowertrain)"
                :key="type"
                type="button"
                @click="downloadTemplate(type)"
                class="py-1.5 px-3 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg flex items-center gap-1.5 border border-slate-700 transition-colors"
              >
                <Download class="w-3.5 h-3.5 text-rose-400" />
                {{ $t(`onboarding.csvTemplate.${type}`) }}
              </button>
            </div>
          </div>

          <p class="text-xs text-slate-400">{{ $t('onboarding.dataSourcesLater') }}</p>

          <div class="flex gap-3 pt-2">
            <button
              type="button"
              @click="currentStep = 2"
              class="py-3 px-4 bg-slate-800 hover:bg-slate-700 text-slate-300 font-semibold rounded-xl text-sm transition-colors"
            >
              {{ $t('onboarding.onboardingView.back') }}
            </button>
            <button
              type="button"
              :disabled="loading"
              @click="handleFinalSubmit"
              class="flex-1 py-3 px-4 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white font-semibold rounded-xl shadow-lg shadow-rose-600/25 flex items-center justify-center gap-2 transition-all disabled:opacity-50"
            >
              <span>{{ loading ? $t('onboarding.saving') : $t('onboarding.finalize') }}</span>
              <ArrowRight class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      <!-- STEP 4: Completed -->
      <div v-else-if="currentStep === 4" class="text-center py-6">
        <div class="inline-flex p-4 bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 rounded-full mb-4">
          <CheckCircle2 class="w-12 h-12" />
        </div>
        <h2 class="text-2xl font-bold text-white mb-2">{{ $t('onboarding.onboardingView.congratulations') }}</h2>
        <p class="text-slate-400 text-sm max-w-sm mx-auto mb-6">
          {{ $t('onboarding.onboardingView.yourAdministratorAccountAndYour') }}
        </p>
        <div v-if="webhook" class="text-left bg-slate-800/60 border border-slate-700 rounded-xl p-4 mb-6 space-y-3">
          <h3 class="text-sm font-semibold text-white">{{ $t('onboarding.webhookReadyTitle') }}</h3>
          <p class="text-xs text-slate-400">{{ $t('onboarding.webhookReadyHint') }}</p>
          <div>
            <div class="flex items-center justify-between mb-1">
              <span class="text-xs font-semibold text-slate-300 uppercase tracking-wider">{{ $t('onboarding.webhookToken') }}</span>
              <button type="button" @click="copy('token')" class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1">
                <Copy class="w-3.5 h-3.5" />{{ copiedField === 'token' ? $t('onboarding.copied') : $t('onboarding.copy') }}
              </button>
            </div>
            <code class="block bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-emerald-300 break-all">{{ webhook.token }}</code>
          </div>
          <div>
            <div class="flex items-center justify-between mb-1">
              <span class="text-xs font-semibold text-slate-300 uppercase tracking-wider">{{ $t('onboarding.webhookExample') }}</span>
              <button type="button" @click="copy('snippet')" class="text-xs text-rose-400 hover:text-rose-300 flex items-center gap-1">
                <Copy class="w-3.5 h-3.5" />{{ copiedField === 'snippet' ? $t('onboarding.copied') : $t('onboarding.copy') }}
              </button>
            </div>
            <pre class="bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-300 overflow-x-auto whitespace-pre">{{ webhook.snippet }}</pre>
          </div>
        </div>
        <p v-else-if="webhookFailed" class="text-xs text-amber-300 bg-amber-500/10 border border-amber-500/20 rounded-xl p-3 mb-6">{{ $t('onboarding.webhookTokenFailed') }}</p>

        <button
          @click="finishOnboarding"
          class="w-full py-3.5 px-6 bg-gradient-to-r from-rose-600 to-rose-500 hover:from-rose-500 hover:to-rose-400 text-white font-semibold rounded-xl shadow-lg shadow-rose-600/25 transition-all"
        >
          {{ $t('onboarding.onboardingView.goToMyDashboard') }}
        </button>
      </div>
    </div>
  </div>
</template>
