<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Zap } from 'lucide-vue-next'

const router = useRouter()
const authStore = useAuthStore()

onMounted(async () => {
  // The API already set the session cookies before redirecting here; just confirm they work.
  if (authStore.status === 'unknown') {
    await authStore.init()
  }
  if (authStore.isAuthenticated) {
    router.replace('/')
  } else {
    router.replace('/login?error=oidc_failed')
  }
})
</script>

<template>
  <div class="min-h-screen flex flex-col items-center justify-center gap-4 bg-slate-950">
    <div class="inline-flex p-3 bg-gradient-to-tr from-rose-500 to-warning-500 rounded-2xl shadow-lg shadow-rose-500/20 animate-pulse">
      <Zap class="w-8 h-8 text-white" />
    </div>
    <p class="text-slate-400 text-sm">{{ $t('auth.oIDCCallbackView.singleSignOnInProgress') }}</p>
  </div>
</template>
