import { expect, test } from '@playwright/test'
import { expectNoHorizontalScroll, useVehicle, vehicles, type Vehicle } from './helpers'

let ev: Vehicle
let ice: Vehicle

test.beforeAll(async ({ request }) => {
  const all = await vehicles(request)
  ev = all.find((v) => v.powertrain === 'EV')!
  ice = all.find((v) => v.powertrain === 'ICE')!
  expect(ev && ice).toBeTruthy()
})

const toast = (page: import('@playwright/test').Page) => page.getByRole('status').filter({ hasText: /\S/ })

test('quick-add records a charge typed with its cost', async ({ page }) => {
  await useVehicle(page, ev.id, '/energy')
  await page.getByRole('button', { name: 'Quick add' }).first().click()
  await page.fill('#qc-kwh', '37.5')
  await page.fill('#qc-cost', '9.80')
  await page.getByRole('button', { name: 'Save the charge' }).click()
  await expect(toast(page)).toContainText('37.5')
})

test('quick-add records a fill-up on a combustion vehicle', async ({ page }) => {
  await useVehicle(page, ice.id, '/energy')
  await page.getByRole('button', { name: 'Quick add' }).first().click()
  await page.fill('#qf-liters', '40')
  await page.fill('#qf-amount', '72')
  await page.getByRole('button', { name: /^save/i }).click()
  await expect(toast(page)).toContainText('72')
})

test('energy tabs follow the powertrain and the legacy fuel URL redirects', async ({ page }) => {
  await useVehicle(page, ice.id, '/manual?tab=FUEL')
  await expect(page).toHaveURL(/\/energy\?tab=FUEL/)
  await expect(page.getByRole('button', { name: 'New fill-up' })).toBeVisible()
  await expect(page.getByRole('tab', { name: 'Charges' })).toHaveCount(0)

  await useVehicle(page, ev.id, '/energy')
  await expect(page.getByRole('tab', { name: 'Charges' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'New fill-up' })).toHaveCount(0)
})

for (const width of [320, 360, 375, 390, 414]) {
  test(`no horizontal scroll at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 800 })
    for (const route of ['/', '/energy', '/expenses', '/maintenance', '/account']) {
      await useVehicle(page, ev.id, route)
      await expectNoHorizontalScroll(page)
    }
  })
}

const modal = (page: import('@playwright/test').Page) => page.getByRole('dialog', { name: 'Add tires' })

test.describe('a modal with edits does not close on a backdrop click', () => {
  test.beforeEach(async ({ page }) => {
    await useVehicle(page, ev.id, '/tires')
    await page.getByRole('button', { name: 'Add tires' }).first().click()
    await expect(modal(page)).toBeVisible()
  })

  test('typed input is kept and Escape still closes', async ({ page }) => {
    await page.fill('#tire-add-tire-brand', 'Acme')
    await page.mouse.click(4, 4)
    await expect(modal(page)).toBeVisible()
    await expect(page.locator('#tire-add-tire-brand')).toHaveValue('Acme')
    await expect(toast(page)).toContainText('Unsaved changes')
    await page.keyboard.press('Escape')
    await expect(modal(page)).toHaveCount(0)
  })

  test('an untouched modal closes on a backdrop click', async ({ page }) => {
    await page.mouse.click(4, 4)
    await expect(modal(page)).toHaveCount(0)
  })
})

test('the dashboard follow-up zone is closed on a phone and opens on demand', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await useVehicle(page, ev.id, '/')
  const toggle = page.getByRole('button', { name: /^Follow-up/ })
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await expect(page.locator('#dashboard-follow-up')).toBeHidden()
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-expanded', 'true')
  await expect(page.locator('#dashboard-follow-up')).toBeVisible()
})

test('the dashboard follow-up zone is open on a wide screen', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await useVehicle(page, ev.id, '/')
  await expect(page.getByRole('button', { name: /^Follow-up/ })).toHaveAttribute('aria-expanded', 'true')
})

test('the odometer page lists the mileage typed on fill-ups, read-only', async ({ page }) => {
  await useVehicle(page, ice.id, '/odometer')
  await expect(page.getByText('Fill-up', { exact: true }).first()).toBeVisible()
  const row = page.locator('div.rounded-xl', { has: page.getByText('Fill-up', { exact: true }) }).first()
  await expect(row.getByRole('button')).toHaveCount(0)
  await expect(row.getByRole('link', { name: 'See the fill-ups' })).toHaveAttribute('href', /\/energy\?tab=FUEL/)
})

test('an electric vehicle lists no fill-up on the odometer page', async ({ page }) => {
  await useVehicle(page, ev.id, '/odometer')
  await expect(page.getByText('History of recorded readings')).toBeVisible()
  await expect(page.getByText('Fill-up', { exact: true })).toHaveCount(0)
})

test('the drive filters sit behind a button on a phone and stay visible on a wide screen', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await useVehicle(page, ev.id, '/drives')
  const toggle = page.getByRole('button', { name: /^Filters/ })
  const withToll = page.getByRole('button', { name: /With toll/ })
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await expect(withToll).toBeHidden()
  await expect(page.getByPlaceholder(/Search for a city/)).toBeVisible()
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-expanded', 'true')
  await withToll.click()
  await expect(toggle).toContainText('1')
  await toggle.click()
  await expect(withToll).toBeHidden()
  await expect(toggle).toContainText('1')

  await page.setViewportSize({ width: 1280, height: 900 })
  await expect(toggle).toBeHidden()
  await expect(withToll).toBeVisible()
})

test('a modal covers the bottom bar of a phone', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 700 })
  await useVehicle(page, ice.id, '/energy?tab=FUEL')
  await page.getByRole('button', { name: 'New fill-up' }).click()
  await expect(page.getByRole('dialog', { name: 'New fill-up' })).toBeVisible()
  const barIsReachable = await page.evaluate(() => {
    const bar = document.querySelector('nav.fixed')!
    const { left, top, width, height } = bar.getBoundingClientRect()
    const hit = document.elementFromPoint(left + width * 0.1, top + height / 2)
    return !!hit && bar.contains(hit)
  })
  expect(barIsReachable).toBe(false)
})
