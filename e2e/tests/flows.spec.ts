import { expect, test } from '@playwright/test'
import { authHeaders, expectNoHorizontalScroll, useVehicle, vehicles, type Vehicle } from './helpers'

test('what-if allows confirming an unusually high energy price without changing the saved scenario', async ({ page, request }) => {
  const headers = await authHeaders(request)
  const created = await request.post('/api/vehicles', { headers, data: { name: 'Price warning reference', powertrain: 'EV', currency: 'EUR' } })
  expect(created.ok()).toBeTruthy()
  const vehicle = await created.json()
  let scenarioId: string | undefined
  try {
    for (let i = 0; i < 5; i++) {
      const charge = await request.post(`/api/vehicles/${vehicle.id}/charges`, { headers, data: { date: '2026-01-01T12:00:00Z', kwh_added: 10, cost: 2, currency: 'EUR' } })
      expect(charge.ok()).toBeTruthy()
    }
    const name = 'Relative price warning'
    const saved = await request.post('/api/comparison-scenarios/', { headers, data: {
      name, mode: 'PROJECTION', currency: 'EUR', annual_km: 12000, years: 5,
      ice: { fuel_type: 'SP95_E10', l_per_100km: 6.5, fuel_price: 1.7 },
      tracked: { kwh_per_100km: 16, eur_per_kwh: 0.2 },
    } })
    expect(saved.ok()).toBeTruthy()
    scenarioId = (await saved.json()).id
    await useVehicle(page, vehicle.id, '/comparison')
    const history = page.waitForResponse(r => r.url().includes('/comparison-scenarios/price-history'))
    await page.getByText(name, { exact: true }).click()
    expect((await history).ok()).toBeTruthy()
    await page.locator('#wi-elec').fill('0.8')
    await expect(page.getByText(/more than 3 times/)).toBeVisible()
    const save = page.getByRole('button', { name: 'Save as a new comparison' })
    await expect(save).toBeDisabled()
    await page.setViewportSize({ width: 320, height: 800 })
    await expectNoHorizontalScroll(page)
    const preview = page.waitForResponse(r => r.url().endsWith('/comparison-scenarios/preview'))
    await page.getByRole('button', { name: 'Keep this price' }).click()
    expect((await preview).ok()).toBeTruthy()
    await expect(save).toBeEnabled()
    await expect(page.locator('#wi-elec')).toHaveValue('0.8')
    await page.locator('#wi-elec').fill('0.9')
    await expect(page.getByRole('button', { name: 'Keep this price' })).toBeVisible()
    const list = await request.get('/api/comparison-scenarios/', { headers })
    const original = (await list.json()).find((s: any) => s.id === scenarioId)
    expect(original.tracked.eur_per_kwh).toBe(0.2)
  } finally {
    if (scenarioId) await request.delete(`/api/comparison-scenarios/${scenarioId}`, { headers })
    await request.delete(`/api/vehicles/${vehicle.id}`, { headers })
  }
})

test('what-if asks before keeping an unusually high fuel price', async ({ page, request }) => {
  const headers = await authHeaders(request)
  const created = await request.post('/api/vehicles', { headers, data: { name: 'Fuel warning reference', powertrain: 'ICE', currency: 'EUR' } })
  expect(created.ok()).toBeTruthy()
  const vehicle = await created.json()
  let scenarioId: string | undefined
  try {
    for (let i = 0; i < 5; i++) {
      const fill = await request.post(`/api/vehicles/${vehicle.id}/fuel-logs`, { headers, data: { date: '2026-01-01T12:00:00Z', amount: 17, liters: 10, fuel_type: 'SP95_E10' } })
      expect(fill.ok()).toBeTruthy()
    }
    const name = 'Relative fuel warning'
    const saved = await request.post('/api/comparison-scenarios/', { headers, data: {
      name, mode: 'PROJECTION', currency: 'EUR', annual_km: 12000, years: 5,
      ice: { fuel_type: 'SP95_E10', l_per_100km: 6.5, fuel_price: 1.7 },
      tracked: { kwh_per_100km: 16, eur_per_kwh: 0.2 },
    } })
    expect(saved.ok()).toBeTruthy()
    scenarioId = (await saved.json()).id
    await useVehicle(page, vehicle.id, '/comparison')
    const history = page.waitForResponse(r => r.url().includes('/comparison-scenarios/price-history'))
    await page.getByText(name, { exact: true }).click()
    expect((await history).ok()).toBeTruthy()
    await page.locator('#wi-fuel').fill('6')
    await expect(page.getByText(/more than 3 times/)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Save as a new comparison' })).toBeDisabled()
    await page.getByRole('button', { name: 'Keep this price' }).click()
    await expect(page.getByRole('button', { name: 'Save as a new comparison' })).toBeEnabled()
    await expect(page.locator('#wi-fuel')).toHaveValue('6')
  } finally {
    if (scenarioId) await request.delete(`/api/comparison-scenarios/${scenarioId}`, { headers })
    await request.delete(`/api/vehicles/${vehicle.id}`, { headers })
  }
})

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

test.describe('the tire modals share one frame', () => {
  test('add and history open named, close on Escape and on the close button', async ({ page }) => {
    await useVehicle(page, ev.id, '/tires')
    await page.getByRole('button', { name: 'Add tires' }).first().click()
    const add = page.getByRole('dialog', { name: 'Add tires' })
    await expect(add).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(add).toHaveCount(0)

    await page.getByRole('button', { name: /^Open Michelin Pilot Sport 4/ }).first().click()
    const history = page.getByRole('dialog', { name: /Michelin Pilot Sport 4/ })
    await expect(history).toBeVisible()
    await expect(history.getByRole('button', { name: 'Edit the tire' })).toBeVisible()
    await history.getByRole('button', { name: 'Close', exact: true }).first().click()
    await expect(history).toHaveCount(0)
  })
})

test('the expense and vehicle modals open named and close on Escape', async ({ page }) => {
  await useVehicle(page, ev.id, '/expenses')
  await page.getByRole('button', { name: 'Add Toll / parking' }).click()
  const toll = page.getByRole('dialog', { name: 'Add a toll / parking fee' })
  await expect(toll).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(toll).toHaveCount(0)

  await page.getByRole('tab', { name: 'Fixed costs' }).click()
  await page.getByRole('button', { name: 'Add Fixed costs' }).click()
  const maintenance = page.getByRole('dialog', { name: /Add maintenance \/ fixed expense/ })
  await expect(maintenance).toBeVisible()
  await maintenance.getByRole('button', { name: 'Close', exact: true }).first().click()
  await expect(maintenance).toHaveCount(0)

  await useVehicle(page, ev.id, '/vehicles')
  await page.locator('button.btn-primary', { has: page.locator('svg') }).first().click()
  const vehicle = page.getByRole('dialog').first()
  await expect(vehicle).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
})

test('a toll added from the cost modal of a drive is listed, edited and deleted', async ({ page, request }) => {
  // A run that stopped half way must not leave its toll behind for the next one
  const headers = await authHeaders(request)
  const leftovers = await (await request.get(`/api/vehicles/${ev.id}/expenses`, { headers })).json()
  for (const e of leftovers.filter((x: { notes?: string }) => x.notes === 'e2e cost modal')) {
    await request.delete(`/api/vehicles/${ev.id}/expenses/${e.id}`, { headers })
  }

  await useVehicle(page, ev.id, '/drives')
  await page.getByRole('button', { name: /^Open the cost breakdown of the drive of/ }).nth(4).click()
  const dialog = page.getByRole('dialog').last()
  await expect(dialog.getByText('0 cost(s) assigned')).toBeVisible()

  await dialog.getByRole('button', { name: 'Add a toll or parking fee to this drive' }).click()
  await dialog.locator('#drive-inline-toll-amount').fill('3.2')
  await dialog.locator('#drive-inline-toll-notes').fill('e2e cost modal')
  await dialog.getByRole('button', { name: 'Confirm', exact: true }).click()
  await expect(dialog.getByText('1 cost(s) assigned')).toBeVisible()
  await expect(dialog.getByText('€3.20').first()).toBeVisible()

  await dialog.getByRole('button', { name: 'Edit this cost' }).click()
  await dialog.locator('[id^="drive-expense-amount-"]').fill('4.5')
  await dialog.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(dialog.getByText('€4.50').first()).toBeVisible()

  await dialog.getByRole('button', { name: 'Delete this cost' }).click()
  await page.getByRole('dialog', { name: 'Delete the cost' }).getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(dialog.getByText('0 cost(s) assigned')).toBeVisible()
})

test('a carpool is entered with its legs and passengers, priced and saved, then deleted', async ({ page, request }) => {
  // The demo carpool of an interrupted run is removed first
  const headers = await authHeaders(request)
  const existing = await (await request.get(`/api/vehicles/${ev.id}/carpools`, { headers })).json()
  for (const trip of (existing.trips ?? []).filter((c: { title: string }) => c.title === 'E2E carpool')) {
    await request.delete(`/api/vehicles/${ev.id}/carpools/${trip.id}`, { headers })
  }

  await useVehicle(page, ev.id, '/carpools')
  await page.getByRole('button', { name: 'New carpool' }).first().click()
  const dialog = page.getByRole('dialog', { name: 'New carpool' })
  await dialog.locator('#carpool-title').fill('E2E carpool')
  await dialog.locator('#leg-start-0').fill('Lyon')
  await dialog.locator('#leg-end-0').fill('Annecy')
  await dialog.locator('#leg-distance-0').fill('100')
  await dialog.locator('#leg-electricity_cost-0').fill('20')
  await dialog.locator('#leg-tolls_cost-0').fill('10')

  // 30 of actual cost shared between the driver and one passenger: the fair share is 15
  await dialog.getByRole('button', { name: 'Apply the fair share' }).click()
  await expect(dialog.locator('#passenger-paid-0')).toHaveValue('15')
  await dialog.getByRole('button', { name: 'Calculation detail' }).click()
  await expect(dialog.getByText('Hide the calculation')).toBeVisible()
  await expect(dialog.getByText('€30.00').first()).toBeVisible()
  await expect(dialog.getByText('€15.00').first()).toBeVisible()

  await dialog.getByRole('button', { name: 'Add a passenger' }).click()
  await expect(dialog.locator('#passenger-name-1')).toBeVisible()
  await dialog.getByRole('button', { name: 'Remove this passenger' }).last().click()
  await expect(dialog.locator('#passenger-name-1')).toHaveCount(0)

  await dialog.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(dialog).toHaveCount(0)
  await expect(page.getByText('E2E carpool').first()).toBeVisible()

  await page.locator('div.rounded-2xl', { hasText: 'E2E carpool' }).getByRole('button', { name: 'Delete', exact: true }).first().click()
  await page.getByRole('dialog').last().getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(page.getByText('E2E carpool')).toHaveCount(0)
})

test('a charges CSV is previewed, imported, then undone from the import history', async ({ page, request }) => {
  const headers = await authHeaders(request)
  const undoLeftovers = async () => {
    const batches = await (await request.get(`/api/vehicles/${ev.id}/import/batches`, { headers })).json()
    for (const b of batches ?? []) await request.delete(`/api/vehicles/${ev.id}/import/batches/${b.id}`, { headers })
  }
  // A run that stopped half way must not leave its batch behind for the next one
  await undoLeftovers()

  await useVehicle(page, ev.id, '/energy?tab=CHARGES')
  await page.getByRole('button', { name: 'Import CSV' }).first().click()
  const dialog = page.getByRole('dialog', { name: 'Import data (CSV)' })
  await dialog.locator('#csv-file-input').setInputFiles({
    name: 'charges.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from('date,kwh,cost,currency,location\n2020-03-04 18:30,12.5,2.5,EUR,E2E csv import\n'),
  })
  await dialog.getByRole('button', { name: 'Preview' }).click()
  await expect(dialog.getByText(/1 valid/i)).toBeVisible()

  await dialog.getByRole('button', { name: 'Import rows' }).click()
  await expect(dialog.getByText('Import completed successfully')).toBeVisible()
  await dialog.getByRole('button', { name: 'Close', exact: true }).last().click()
  await expect(page.getByRole('dialog')).toHaveCount(0)

  const charges = await (await request.get(`/api/vehicles/${ev.id}/charges?limit=500`, { headers })).json()
  expect(JSON.stringify(charges)).toContain('E2E csv import')

  // The import history of the dialog undoes the batch
  await page.getByRole('button', { name: 'Import CSV' }).first().click()
  const history = page.getByRole('dialog', { name: 'Import data (CSV)' })
  await history.getByText('Import history').click()
  await history.getByRole('button', { name: 'Undo this import' }).first().click()
  await page.getByRole('dialog').last().getByRole('button', { name: 'Undo this import' }).click()
  await expect.poll(async () => JSON.stringify(await (await request.get(`/api/vehicles/${ev.id}/charges?limit=500`, { headers })).json())).not.toContain('E2E csv import')
})

test.describe('a charge priced from a tariff plan', () => {
  // The server reads tariff hours in its reporting timezone (Europe/Paris by default): the browser uses the same one
  test.use({ timezoneId: 'Europe/Paris' })

  test('is priced hour by hour when it ends after midnight', async ({ page, request }) => {
    const headers = await authHeaders(request)
    const vehicleUrl = `/api/vehicles/${ev.id}`
    const vehicle = await (await request.get(vehicleUrl, { headers })).json()
    const plansUrl = '/api/tariffs/plans'
    const dropPlans = async () => {
      await request.put(vehicleUrl, { headers, data: { ...vehicle, tariff_plan_id: null } })
      const { plans } = await (await request.get(plansUrl, { headers })).json()
      for (const p of plans.filter((x: { name: string }) => x.name === 'E2E night')) await request.delete(`${plansUrl}/${p.id}`, { headers })
    }
    const dropCharges = async () => {
      const list = await (await request.get(`${vehicleUrl}/charges?limit=500`, { headers })).json()
      for (const c of (list.charges ?? list).filter((x: { notes?: string }) => x.notes === 'e2e tariff midnight')) {
        await request.delete(`${vehicleUrl}/charges/${c.id}`, { headers })
      }
    }
    // A run that stopped half way must not leave its plan or charge behind for the next one
    await dropCharges()
    await dropPlans()

    try {
      // 0.30 a kWh by day, 0.10 from midnight to 06:00
      const created = await request.post(plansUrl, {
        headers,
        data: {
          name: 'E2E night',
          plan_type: 'BANDS',
          currency: 'EUR',
          default_band: 'day',
          bands: [{ name: 'day', rate_cents: 0.3 }, { name: 'night', rate_cents: 0.1 }],
          rules: [{ start: '00:00', end: '06:00', band: 'night' }],
        },
      })
      expect(created.ok()).toBeTruthy()
      const plan = await created.json()
      expect((await request.put(vehicleUrl, { headers, data: { ...vehicle, tariff_plan_id: plan.id } })).ok()).toBeTruthy()

      // The form starts at 23:00 on 4 March 2020
      await page.clock.setFixedTime(new Date('2020-03-04T23:00:00+01:00'))
      await useVehicle(page, ev.id, '/energy?tab=CHARGES')
      await page.getByRole('button', { name: 'Add a charge' }).first().click()
      const dialog = page.getByRole('dialog', { name: 'New charge' })
      await expect(dialog.locator('input[aria-label="Datepicker input"]').first()).toHaveValue('04/03/2020 23:00')

      // It ends at 01:00 the next day
      await dialog.locator('input[aria-label="Datepicker input"]').nth(1).click()
      const picker = page.locator('.dp--menu')
      await picker.getByLabel('March 5th, 2020').click()
      await picker.getByLabel('Open time picker').click()
      await picker.getByLabel('Increment hours').click({ clickCount: 2 })
      await expect(picker.getByText('05/03/2020 01:00')).toBeVisible()
      await picker.getByRole('button', { name: 'Apply' }).click()

      // One hour on each side of midnight: 10 kWh at 0.30 and 10 kWh at 0.10
      await dialog.locator('#charge-form-kwh').fill('20')
      await expect(dialog.locator('#charge-form-cost')).toHaveValue('4')
      await expect(dialog.getByText('E2E night').first()).toBeVisible()

      await dialog.locator('#charge-form-notes').fill('e2e tariff midnight')
      await dialog.getByRole('button', { name: 'Save', exact: true }).click()
      await expect(dialog).toHaveCount(0)
      const list = await (await request.get(`${vehicleUrl}/charges?limit=500`, { headers })).json()
      const saved = (list.charges ?? list).find((x: { notes?: string }) => x.notes === 'e2e tariff midnight')
      expect(saved.cost).toBe(4)
    } finally {
      await dropCharges()
      await dropPlans()
    }
  })
})

test('a session from a shared charger is qualified onto a vehicle', async ({ page, request }) => {
  const headers = await authHeaders(request)
  const dropSecondVehicle = async () => {
    const body = await (await request.get('/api/vehicles', { headers })).json()
    for (const v of (Array.isArray(body) ? body : body.vehicles).filter((x: Vehicle) => x.name === 'E2E second EV')) {
      await request.delete(`/api/vehicles/${v.id}`, { headers })
    }
  }
  const dropPending = async () => {
    const { pending_charges: pending } = await (await request.get('/api/pending-charges', { headers })).json()
    for (const p of (pending ?? []).filter((x: { energy_kwh: number }) => x.energy_kwh === 17.25)) {
      await request.delete(`/api/pending-charges/${p.id}`, { headers })
    }
  }
  // A run that stopped half way must not leave its vehicle or its session behind for the next one
  await dropSecondVehicle()
  await dropPending()

  // With two electric vehicles, a session that names none waits for the household to say whose it is
  const second = await request.post('/api/vehicles', { headers, data: { name: 'E2E second EV', powertrain: 'EV', currency: 'EUR', current_odometer: 0 } })
  expect(second.ok()).toBeTruthy()
  try {
    const sent = await request.post('/api/integrations/homeassistant/event', {
      headers,
      data: {
        event_id: `e2e-wallbox:${Date.now()}`,
        event_type: 'charging_session_end',
        data: { start_time: '2020-03-04T20:00:00Z', end_time: '2020-03-04T22:00:00Z', energy_added_kwh: 17.25 },
      },
    })
    expect((await sent.json()).status).toBe('pending_qualification')

    await useVehicle(page, ev.id, '/energy?tab=CHARGES')
    await page.getByRole('button', { name: 'Qualify' }).click()
    const dialog = page.getByRole('dialog', { name: 'Charges to Qualify' })
    await expect(dialog.getByText('17.25 kWh')).toBeVisible()
    await dialog.getByLabel('Select vehicle').selectOption({ label: ev.name })
    await dialog.getByRole('button', { name: 'Assign' }).click()
    await expect(dialog.getByText('17.25 kWh')).toHaveCount(0)

    // The session is now a charge of the chosen vehicle
    const list = await (await request.get(`/api/vehicles/${ev.id}/charges?limit=500`, { headers })).json()
    const charge = (list.charges ?? list).find((c: { kwh_added: number }) => c.kwh_added === 17.25)
    expect(charge).toBeTruthy()
    await request.delete(`/api/vehicles/${ev.id}/charges/${charge.id}`, { headers })
  } finally {
    await dropPending()
    await dropSecondVehicle()
  }
})
