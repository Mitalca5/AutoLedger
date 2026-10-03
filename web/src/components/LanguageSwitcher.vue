<script setup lang="ts">
import { computed } from 'vue'
import { Languages } from 'lucide-vue-next'
import { currentLocale, setLocale, SUPPORTED_LOCALES, type AppLocale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/services/api'

// Each language is named in itself, so it stays readable whatever the current language is.
const LANGUAGE_NAMES: Record<AppLocale, string> = { en: 'English', fr: 'Français' }

const authStore = useAuthStore()

const selected = computed({
  get: () => currentLocale(),
  set: (value: AppLocale) => {
    setLocale(value)
    // Reminder and sync-failure webhooks are built outside any request: the server needs its
    // own copy of the choice. Best-effort — the UI itself already switched.
    if (authStore.isAuthenticated) api.updateLanguage(value).catch(() => {})
  },
})
</script>

<template>
  <label class="flex items-center gap-1.5 text-xs text-slate-400">
    <Languages class="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
    <span class="sr-only">{{ $t('shell.language.label') }}</span>
    <select
      v-model="selected"
      class="bg-slate-900 border border-slate-700 rounded-md px-1.5 py-0.5 text-xs text-slate-200 focus:outline-none focus:border-rose-500"
    >
      <option v-for="locale in SUPPORTED_LOCALES" :key="locale" :value="locale">{{ LANGUAGE_NAMES[locale] }}</option>
    </select>
  </label>
</template>
