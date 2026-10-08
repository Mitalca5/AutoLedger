import { expect, type APIRequestContext, type Page } from '@playwright/test'

export const email = process.env.DEMO_EMAIL ?? 'demo@autoledger.example'
export const password = process.env.DEMO_PASSWORD ?? ''

export interface Vehicle {
  id: string
  name: string
  powertrain: 'EV' | 'ICE' | 'PHEV' | 'REEV'
}

export async function vehicles(request: APIRequestContext): Promise<Vehicle[]> {
  const login = await request.post('/api/auth/login', { data: { email, password } })
  expect(login.ok()).toBeTruthy()
  const { token, access_token: accessToken } = await login.json()
  const res = await request.get('/api/vehicles', { headers: { Authorization: `Bearer ${token ?? accessToken}` } })
  const body = await res.json()
  return Array.isArray(body) ? body : body.vehicles
}

export const authFile = '.auth/state.json'

export async function login(page: Page) {
  await page.addInitScript(() => localStorage.setItem('teslacost_locale', 'en'))
  await page.goto('/login')
  await page.fill('#login-email', email)
  await page.fill('#login-password', password)
  await page.locator('button[type=submit]').click()
  await page.waitForURL((url) => !url.pathname.includes('login'))
}

// Opens the app as the signed-in demo user with the given vehicle active (the key is read at startup).
export async function useVehicle(page: Page, vehicleId: string, route = '/') {
  await page.goto('/login')
  await page.evaluate(([id]) => {
    localStorage.setItem('teslacost_locale', 'en')
    localStorage.setItem('teslacost_active_vehicle', id)
  }, [vehicleId])
  await page.goto(route)
  await page.waitForLoadState('networkidle')
}

export async function expectNoHorizontalScroll(page: Page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
  expect(overflow).toBeLessThanOrEqual(0)
}
