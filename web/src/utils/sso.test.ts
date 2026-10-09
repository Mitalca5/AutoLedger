import { describe, expect, it } from 'vitest'
import { ssoLinkNotice, ssoLoginErrorKey } from './sso'

describe('ssoLinkNotice', () => {
  it('turns every outcome the server sends into a message', () => {
    expect(ssoLinkNotice('linked')).toEqual({ key: 'account.sso.linkedNotice', tone: 'success' })
    expect(ssoLinkNotice('already_linked')?.tone).toBe('success')
    for (const code of ['identity_in_use', 'email_mismatch', 'failed']) {
      expect(ssoLinkNotice(code)?.tone).toBe('warning')
    }
  })
  it('shows nothing for anything else, so a made-up value in the address bar prints nothing', () => {
    expect(ssoLinkNotice(undefined)).toBeNull()
    expect(ssoLinkNotice('<script>')).toBeNull()
    expect(ssoLinkNotice(['linked'])).toBeNull()
  })
})

describe('ssoLoginErrorKey', () => {
  it('names the message of each sign-in error', () => {
    expect(ssoLoginErrorKey('oidc_failed')).toBe('auth.loginView.ssoFailed')
    expect(ssoLoginErrorKey('oidc_local_account_exists')).toBe('auth.loginView.ssoLocalAccountExists')
  })
  it('ignores unknown errors', () => {
    expect(ssoLoginErrorKey('whatever')).toBeNull()
    expect(ssoLoginErrorKey(null)).toBeNull()
  })
})
