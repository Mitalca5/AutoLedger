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
