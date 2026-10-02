const envVersion = import.meta.env.VITE_APP_VERSION
export const APP_VERSION = envVersion
  ? (envVersion.startsWith('v') ? envVersion : `v${envVersion}`)
  : 'v1.18.0'
