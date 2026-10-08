import { test as setup } from '@playwright/test'
import { authFile, login } from './helpers'

setup('sign in once', async ({ page }) => {
  await login(page)
  await page.context().storageState({ path: authFile })
})
