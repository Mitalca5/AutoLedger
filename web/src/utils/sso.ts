export interface SsoNotice {
  key: string
  tone: 'success' | 'warning'
}

/** The message shown on the account page when the server sends the user back after linking a single sign-on (`?sso=`). */
export function ssoLinkNotice(code: unknown): SsoNotice | null {
  switch (code) {
    case 'linked':
      return { key: 'account.sso.linkedNotice', tone: 'success' }
    case 'already_linked':
      return { key: 'account.sso.alreadyLinked', tone: 'success' }
    case 'identity_in_use':
      return { key: 'account.sso.identityInUse', tone: 'warning' }
    case 'email_mismatch':
      return { key: 'account.sso.emailMismatch', tone: 'warning' }
    case 'failed':
      return { key: 'account.sso.failed', tone: 'warning' }
    default:
      return null
  }
}

/** The message of the login page after a single sign-on that did not sign the user in (`?error=`). */
export function ssoLoginErrorKey(code: unknown): string | null {
  switch (code) {
    case 'oidc_failed':
      return 'auth.loginView.ssoFailed'
    case 'oidc_local_account_exists':
      return 'auth.loginView.ssoLocalAccountExists'
    default:
      return null
  }
}
