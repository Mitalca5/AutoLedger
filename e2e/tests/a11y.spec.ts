import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { useVehicle, vehicles, type Vehicle } from './helpers'

let ev: Vehicle
let ice: Vehicle

test.beforeAll(async ({ request }) => {
  const all = await vehicles(request)
  ev = all.find((v) => v.powertrain === 'EV')!
  ice = all.find((v) => v.powertrain === 'ICE')!
  expect(ev && ice).toBeTruthy()
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

test('the sidebar is a named landmark and marks the current page', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await useVehicle(page, ev.id, '/tires')
  const nav = page.getByRole('navigation', { name: 'Main navigation' })
  await expect(nav).toBeVisible()
  await expect(nav.locator('a[aria-current="page"]')).toHaveCount(1)
  await expect(nav.locator('a[aria-current="page"]')).toHaveAttribute('href', '/tires')
  await expect(nav.getByRole('group', { name: 'Tracking' }).getByRole('link', { name: 'Drives' })).toBeVisible()
  await expect(nav.getByRole('group', { name: 'Costs' }).getByRole('link', { name: 'Tires' })).toBeVisible()
  await expect(nav.getByRole('group', { name: 'Management' }).getByRole('link', { name: 'Account' })).toBeVisible()
})

test('a combustion vehicle sidebar has no empty section and no drives page', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await useVehicle(page, ice.id, '/odometer')
  const nav = page.getByRole('navigation', { name: 'Main navigation' })
  await expect(nav.getByRole('group', { name: 'Tracking' }).getByRole('link', { name: 'Odometer' })).toBeVisible()
  await expect(nav.getByRole('link', { name: 'Drives', exact: true })).toHaveCount(0)
})

test('a focused field shows a visible ring', async ({ page }) => {
  await useVehicle(page, ev.id, '/tires')
  await page.getByRole('button', { name: 'Add tires' }).first().click()
  const brand = page.locator('#tire-add-tire-brand')
  await brand.focus()
  const ring = await brand.evaluate((el) => getComputedStyle(el).boxShadow)
  expect(ring).not.toBe('none')
})

test('the pages of a combustion vehicle have no WCAG 2.1 AA violation', async ({ page }) => {
  for (const route of ['/', '/odometer', '/energy?tab=FUEL', '/maintenance']) {
    await useVehicle(page, ice.id, route)
    const { violations } = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
    expect(violations.map((v) => `${route} ${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)).toEqual([])
  }
})
