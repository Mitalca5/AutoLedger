import { createRouter, createWebHistory, type RouteComponent, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Every view is its own chunk, loaded on first navigation: the sign-in page does not download the whole app.
// A page that needs a signed-in session.
const authRoute = (path: string, name: string, component: RouteComponent): RouteRecordRaw => ({
  path,
  name,
  component,
  meta: { requiresAuth: true },
})

const router = createRouter({
  history: createWebHistory(),
  routes: [
    authRoute('/', 'dashboard', () => import('@/views/DashboardView.vue')),
    authRoute('/fleet', 'fleet', () => import('@/views/FleetView.vue')),
    authRoute('/drives', 'drives', () => import('@/views/DrivesView.vue')),
    authRoute('/carpools', 'carpools', () => import('@/views/CarpoolView.vue')),
    authRoute('/odometer', 'odometer', () => import('@/views/ManualTrackingView.vue')),
    {
      // Former address of the odometer page; the energy estimate and the fill-ups moved to the energy page
      path: '/manual',
      redirect: (to) => {
        const tab = String(to.query.tab ?? '').toUpperCase()
        if (tab === 'ENERGY') return { path: '/energy', query: { tab: 'ESTIMATE' } }
        if (tab === 'FUEL') return { path: '/energy', query: { tab: 'FUEL' } }
        return { path: '/odometer' }
      },
    },
    authRoute('/comparison', 'comparison', () => import('@/views/ComparisonView.vue')),
    authRoute('/tires', 'tires', () => import('@/views/TiresView.vue')),
    {
      path: '/expenses',
      name: 'expenses',
      component: () => import('@/views/ExpensesView.vue'),
      meta: { requiresAuth: true },
      // Maintenance and reminders moved to their own menu entry
      beforeEnter: (to) => {
        const tab = String(to.query.tab ?? '').toUpperCase()
        if (tab === 'MAINTENANCE' || tab === 'REMINDERS') return { name: 'maintenance', query: to.query, replace: true }
        if (tab === 'CHARGES') return { name: 'energy', query: to.query, replace: true }
      },
    },
    {
      path: '/maintenance',
      name: 'maintenance',
      component: () => import('@/views/ExpensesView.vue'),
      meta: { requiresAuth: true, section: 'maintenance' },
      beforeEnter: (to) => {
        const tab = String(to.query.tab ?? '').toUpperCase()
        if (['TOLLS', 'DOCUMENTS'].includes(tab)) return { name: 'expenses', query: to.query, replace: true }
        if (['CHARGES', 'EFFICIENCY', 'ESTIMATE', 'FUEL'].includes(tab)) return { name: 'energy', query: to.query, replace: true }
      },
    },
    {
      path: '/energy',
      name: 'energy',
      component: () => import('@/views/ExpensesView.vue'),
      meta: { requiresAuth: true, section: 'energy' },
    },
    authRoute('/vehicles', 'vehicles', () => import('@/views/VehiclesView.vue')),
    authRoute('/account', 'account', () => import('@/views/AccountView.vue')),
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/views/RegisterView.vue'),
    },
    {
      path: '/onboarding',
      name: 'onboarding',
      component: () => import('@/views/OnboardingView.vue'),
    },
    {
      // OIDC SSO callback — the API has already set the session cookies before redirecting here.
      path: '/oidc-callback',
      name: 'oidc-callback',
      component: () => import('@/views/OIDCCallbackView.vue'),
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

// A new deployment removes the hashed chunks of the previous build: a navigation that fails to fetch one reloads the
// page on the target route once, which gets the current index and its chunks.
router.onError((error, to) => {
  const message = String((error as Error)?.message ?? error)
  if (!/Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed/i.test(message)) return
  const key = 'al-chunk-reload'
  try {
    if (sessionStorage.getItem(key) === to.fullPath) return
    sessionStorage.setItem(key, to.fullPath)
  } catch {
    // storage blocked: reload anyway, a second failure is not retried by the browser's cache-busting reload
  }
  window.location.assign(to.fullPath)
})

router.afterEach(() => {
  try {
    sessionStorage.removeItem('al-chunk-reload')
  } catch {
    // storage blocked
  }
})

export default router
