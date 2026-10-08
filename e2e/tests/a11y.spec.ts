import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { useVehicle, vehicles, type Vehicle } from './helpers'

let ev: Vehicle

test.beforeAll(async ({ request }) => {
  ev = (await vehicles(request)).find((v) => v.powertrain === 'EV')!
  expect(ev).toBeTruthy()
})

const routes = ['/', '/fleet', '/drives', '/carpools', '/odometer', '/energy', '/comparison', '/tires', '/expenses', '/maintenance', '/vehicles', '/account']

for (const width of [1280, 375]) {
  test(`no WCAG 2.1 AA violation at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    for (const route of routes) {
      await useVehicle(page, ev.id, route)
      const { violations } = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
      expect(violations.map((v) => `${route} ${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)).toEqual([])
    }
  })
}

test('a drive card opens from its button and its checkbox does not open it', async ({ page }) => {
  await useVehicle(page, ev.id, '/drives')
  const card = page.getByRole('button', { name: /^Open the cost breakdown of the drive of/ }).first()
  await card.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)

  await page.getByRole('checkbox', { name: 'Select this drive' }).first().check()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await card.click()
  await expect(page.getByRole('dialog')).toBeVisible()
})
