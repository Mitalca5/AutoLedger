import { coordinateRefresh, refreshVersion } from './sessionRefreshLock'

// All authenticated transfers share one refresh and retry the original request at most once.
// Retrying with the same options also preserves an offline mutation's Idempotency-Key.
let refreshPromise: Promise<boolean> | null = null

export function isPublicAuthRequest(url: string): boolean {
  return ['/api/auth/login', '/api/auth/register', '/api/auth/refresh', '/api/auth/config']
    .some((path) => url.startsWith(path))
}

async function refreshSession(version: string | null): Promise<boolean> {
  if (refreshPromise) return refreshPromise
  refreshPromise = (async () => {
    try {
      return await coordinateRefresh(version, async () => {
        const response = await fetch('/api/auth/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
        })
        return response.ok
      })
    } catch {
      return false
    } finally {
      refreshPromise = null
    }
  })()
  return refreshPromise
}

export async function fetchWithSessionRefresh(url: string, options: RequestInit = {}, canRetry: () => boolean = () => true): Promise<Response> {
  const init = { ...options, credentials: options.credentials || 'include' } as RequestInit
  const version = refreshVersion()
  const response = await fetch(url, init)
  if (response.status === 401 && !isPublicAuthRequest(url) && canRetry() && await refreshSession(version) && canRetry()) {
    return fetch(url, init)
  }
  return response
}
