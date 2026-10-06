import { ref } from 'vue'

// Runs one async action at a time: a second call while the first is pending is ignored, so a double click or a
// double Enter cannot send the same write twice. `pending` drives the :disabled state of the submit button.
export function useSubmit() {
  const pending = ref(false)
  async function run<T>(action: () => Promise<T>): Promise<T | undefined> {
    if (pending.value) return undefined
    pending.value = true
    try {
      return await action()
    } finally {
      pending.value = false
    }
  }
  return { pending, run }
}
