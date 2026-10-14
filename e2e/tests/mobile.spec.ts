import { expect, test } from '@playwright/test'
import { useVehicle, vehicles, type Vehicle } from './helpers'

// A phone: touch pointer, so the 44px hit areas and the 16px fields of main.css apply.
test.use({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true })

let ev: Vehicle

test.beforeAll(async ({ request }) => {
  ev = (await vehicles(request)).find((v) => v.powertrain === 'EV')!
  expect(ev).toBeTruthy()
})

for (const route of ['/drives', '/carpools', '/comparison', '/tires', '/maintenance']) {
  test(`every checkbox has a 44px hit area on ${route}`, async ({ page }) => {
    await useVehicle(page, ev.id, route)
    const small = await page.evaluate(() =>
      [...document.querySelectorAll<HTMLInputElement>('input[type=checkbox]:not([role=switch])')]
        .filter((e) => e.getBoundingClientRect().width > 0 && !e.closest('.sr-only'))
        .map((e) => ({ id: e.id || e.getAttribute('aria-label') || '', box: (e.closest('label, .tap') ?? e).getBoundingClientRect() }))
        .filter((x) => x.box.height < 44)
        .map((x) => `${x.id} ${Math.round(x.box.width)}x${Math.round(x.box.height)}`),
    )
    expect(small).toEqual([])
  })
}

test('a date field inside a form is 16px so the page does not zoom on focus', async ({ page }) => {
  await useVehicle(page, ev.id, '/expenses')
  await page.getByRole('button', { name: /^Add/ }).first().click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  const sizes = await dialog.locator('input.dp--input').evaluateAll((els) => els.map((e) => getComputedStyle(e).fontSize))
  expect(sizes.length).toBeGreaterThan(0)
  expect(sizes).toEqual(sizes.map(() => '16px'))
})
