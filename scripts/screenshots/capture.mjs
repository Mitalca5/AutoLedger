// Captures the documentation screenshots from a running instance seeded with cmd/demoseed.
// Usage: BASE_URL=http://localhost:8080 DEMO_EMAIL=... DEMO_PASSWORD=... OUT_DIR=../../docs/screenshots node capture.mjs
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'

const base = (process.env.BASE_URL ?? 'http://localhost:8080').replace(/\/$/, '')
const email = process.env.DEMO_EMAIL ?? 'demo@autoledger.example'
const password = process.env.DEMO_PASSWORD
const out = process.env.OUT_DIR ?? '../../docs/screenshots'
if (!password) throw new Error('DEMO_PASSWORD is required')
mkdirSync(out, { recursive: true })

const locales = ['en', 'fr']
const desktop = { viewport: { width: 1440, height: 900 } }
const mobile = { viewport: { width: 390, height: 800 }, deviceScaleFactor: 2, hasTouch: true, isMobile: true }

// [file name, route, vehicle powertrain to select, extra wait selector]
const desktopShots = [
  ['dashboard-ev', '/', 'EV'],
  ['dashboard-ice', '/', 'ICE'],
  ['fleet', '/fleet', 'EV'],
  ['drives', '/drives', 'EV'],
  ['expenses', '/expenses', 'EV'],
  ['tires', '/tires', 'EV'],
]

async function session(browser, options, locale) {
  const context = await browser.newContext({ ...options, locale: locale === 'fr' ? 'fr-FR' : 'en-GB', colorScheme: 'dark' })
  const res = await context.request.post(`${base}/api/auth/login`, { data: { email, password } })
  if (!res.ok()) throw new Error(`login failed: ${res.status()}`)
  const token = (await res.json()).token ?? (await res.json()).access_token
  const list = await context.request.get(`${base}/api/vehicles`, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
  const vehicles = await list.json()
  const page = await context.newPage()
  await page.addInitScript((l) => localStorage.setItem('teslacost_locale', l), locale)
  return { context, page, vehicles: Array.isArray(vehicles) ? vehicles : vehicles.vehicles }
}

async function open(page, vehicles, route, powertrain) {
  const vehicle = vehicles.find((v) => v.powertrain === powertrain) ?? vehicles[0]
  await page.goto(`${base}/login`)
  await page.evaluate((id) => localStorage.setItem('teslacost_active_vehicle', id), vehicle.id)
  await page.goto(`${base}${route}`)
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(1500)
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined })
try {
  for (const locale of locales) {
    const d = await session(browser, desktop, locale)
    for (const [name, route, powertrain] of desktopShots) {
      await open(d.page, d.vehicles, route, powertrain)
      await d.page.screenshot({ path: `${out}/${name}.${locale}.png` })
    }
    await d.context.close()

    const m = await session(browser, mobile, locale)
    await open(m.page, m.vehicles, '/', 'EV')
    await m.page.screenshot({ path: `${out}/mobile-dashboard.${locale}.png` })
    await m.page.getByRole('button', { name: locale === 'fr' ? /saisie rapide/i : /quick add/i }).first().click()
    await m.page.waitForTimeout(600)
    await m.page.screenshot({ path: `${out}/mobile-quickadd.${locale}.png` })
    await m.context.close()
  }
} finally {
  await browser.close()
}
