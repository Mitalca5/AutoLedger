const lockName = 'autoledger-session-refresh'
const versionKey = 'autoledger:session-refresh-version'

// This is a coordination marker only; no credential is stored outside HttpOnly cookies.
export function refreshVersion(): string | null {
  try {
    return globalThis.localStorage?.getItem(versionKey) ?? null
  } catch {
    return null
  }
}

function markRefreshed() {
  try {
    globalThis.localStorage?.setItem(versionKey, crypto.randomUUID())
  } catch {
    // Cookies still rotate safely under the lock when storage is unavailable.
  }
}

/** Serialize refresh requests across same-origin tabs; their cookies are shared too. */
export async function coordinateRefresh(version: string | null, refresh: () => Promise<boolean>): Promise<boolean> {
  const run = async () => {
    if (refreshVersion() !== version) return true
    const ok = await refresh()
    if (ok) markRefreshed()
    return ok
  }
  // Web Locks requires a secure context. Older/plain-HTTP browsers keep the per-tab guard.
  const locks = globalThis.navigator?.locks
  return locks ? locks.request(lockName, run) : refresh()
}
