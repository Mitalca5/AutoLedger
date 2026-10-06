import { ref } from 'vue'

export interface Toast {
  id: number
  message: string
  title?: string
}

const TOAST_MS = 4000
const toasts = ref<Toast[]>([])
let nextId = 1

export function useToast() {
  function dismissToast(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  function showToast(message: string, title?: string) {
    const id = nextId++
    toasts.value = [...toasts.value.slice(-2), { id, message, title }]
    setTimeout(() => dismissToast(id), TOAST_MS)
  }

  return { toasts, showToast, dismissToast }
}
