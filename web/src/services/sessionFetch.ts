// All authenticated transfers share one refresh and retry the original request at most once.
// Retrying with the same options also preserves an offline mutation's Idempotency-Key.
let refreshPromise: Promise<boolean> | null = null

export function isPublicAuthRequest(url: string): boolean {
  return ['/api/auth/login', '/api/auth/register', '/api/auth/refresh', '/api/auth/config']
    .some((path) => url.startsWith(path))
}

async function refreshSession(): Promise<boolean> {
  if (refreshPromise) return refreshPromise
  refreshPromise = (async () => {
    try {
      const response = await fetch('/api/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
      })
      return response.ok
    } catch {
      return false
    } finally {
      refreshPromise = null
    }
  })()
  return refreshPromise
}

export async function fetchWithSessionRefresh(url: string, options: RequestInit = {}): Promise<Response> {
  const init = { ...options, credentials: options.credentials || 'include' } as RequestInit
  const response = await fetch(url, init)
  if (response.status === 401 && !isPublicAuthRequest(url) && await refreshSession()) {
    return fetch(url, init)
  }
  return response
}
