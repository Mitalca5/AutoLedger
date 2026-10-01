import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

import DashboardView from '@/views/DashboardView.vue'
import FleetView from '@/views/FleetView.vue'
import DrivesView from '@/views/DrivesView.vue'
import TiresView from '@/views/TiresView.vue'
import ExpensesView from '@/views/ExpensesView.vue'
import VehiclesView from '@/views/VehiclesView.vue'
import AccountView from '@/views/AccountView.vue'
import CarpoolView from '@/views/CarpoolView.vue'
import ComparisonView from '@/views/ComparisonView.vue'
import ManualTrackingView from '@/views/ManualTrackingView.vue'
import LoginView from '@/views/LoginView.vue'
import RegisterView from '@/views/RegisterView.vue'
import OnboardingView from '@/views/OnboardingView.vue'
import OIDCCallbackView from '@/views/OIDCCallbackView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: DashboardView,
      meta: { requiresAuth: true },
    },
    {
      path: '/fleet',
      name: 'fleet',
      component: FleetView,
      meta: { requiresAuth: true },
    },
    {
      path: '/drives',
      name: 'drives',
      component: DrivesView,
      meta: { requiresAuth: true },
    },
    {
      path: '/carpools',
      name: 'carpools',
      component: CarpoolView,
      meta: { requiresAuth: true },
    },
    {
      path: '/manual',
      name: 'manual',
      component: ManualTrackingView,
      meta: { requiresAuth: true },
    },
    {
      path: '/comparison',
      name: 'comparison',
      component: ComparisonView,
      meta: { requiresAuth: true },
    },
    {
      path: '/tires',
      name: 'tires',
      component: TiresView,
      meta: { requiresAuth: true },
    },
    {
      path: '/expenses',
      name: 'expenses',
      component: ExpensesView,
      meta: { requiresAuth: true },
    },
    {
      path: '/vehicles',
      name: 'vehicles',
      component: VehiclesView,
      meta: { requiresAuth: true },
    },
    {
      path: '/account',
      name: 'account',
      component: AccountView,
      meta: { requiresAuth: true },
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/register',
      name: 'register',
      component: RegisterView,
    },
    {
      path: '/onboarding',
      name: 'onboarding',
      component: OnboardingView,
    },
    {
      // OIDC SSO callback — the API has already set the session cookies before redirecting here.
      path: '/oidc-callback',
      name: 'oidc-callback',
      component: OIDCCallbackView,
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

let checkedOnboarding = false
let needsOnboarding = false

async function checkOnboardingStatus() {
  if (checkedOnboarding) return needsOnboarding
  try {
    const res = await fetch('/api/auth/config')
    if (res.ok) {
      const data = await res.json()
      needsOnboarding = !!data.needs_onboarding
      checkedOnboarding = true
    }
  } catch {
    // fallback
  }
  return needsOnboarding
}

router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // The access token lives in an HttpOnly cookie the SPA can't read directly, so the very
  // first navigation of a page load must ask the API whether the session cookie is valid
  // before it can decide where to route. Subsequent navigations reuse the resolved status.
  if (authStore.status === 'unknown') {
    await authStore.init()
  }

  // On first startup without account, route to onboarding
  if (!authStore.isAuthenticated) {
    const isFirstRun = await checkOnboardingStatus()
    if (isFirstRun && to.name !== 'onboarding') {
      return next({ name: 'onboarding' })
    }
  }

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next({ name: 'login' })
  } else if (
    (to.name === 'login' || to.name === 'register' || to.name === 'onboarding') &&
    authStore.isAuthenticated
  ) {
    next({ name: 'dashboard' })
  } else {
    next()
  }
})

export default router
