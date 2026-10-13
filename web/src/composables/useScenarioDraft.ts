import { computed, onScopeDispose, reactive, ref, watch } from 'vue'

export interface ScenarioDraftOptions<T extends object> {
  // The reference values: the saved scenario. The draft starts from them and returns to them on reset.
  base: () => T
  // The draft is in range for the API; anything else is never sent.
  isValid: (draft: T) => boolean
  // The request body of a preview of the draft.
  payload: (draft: T) => unknown
  preview: (payload: unknown) => Promise<any>
  // A pending confirmation (an unusual price): the previous result is dropped and no request is sent.
  blocked?: () => boolean
  debounceMs?: number
}

// An unsaved draft of a scenario: the numeric inputs (an object of numbers) the user is exploring, compared with the saved ones, and the
// server's evaluation of the draft. The latest answer wins and nothing is sent while the draft is unchanged, out of
// range or waiting for a confirmation. Nothing is stored.
export function useScenarioDraft<T extends object>(options: ScenarioDraftOptions<T>) {
  const debounceMs = options.debounceMs ?? 250
  const draft = reactive({ ...options.base() }) as T
  const preview = ref<any | null>(null)
  const loading = ref(false)
  const failed = ref(false)

  const dirty = computed(() => {
    const saved = options.base() as Record<string, unknown>
    const current = draft as Record<string, unknown>
    return Object.keys(saved).some((k) => Number(current[k]) !== Number(saved[k]))
  })
  const valid = computed(() => options.isValid(draft))
  const blocked = computed(() => options.blocked?.() ?? false)

  let timer: ReturnType<typeof setTimeout> | undefined
  let sequence = 0

  function discard() {
    sequence += 1
    preview.value = null
    loading.value = false
    failed.value = false
  }

  function reset() {
    Object.assign(draft, options.base())
  }

  async function refresh() {
    const current = ++sequence
    loading.value = true
    failed.value = false
    try {
      const res = await options.preview(options.payload(draft))
      if (current === sequence) preview.value = res
    } catch (err) {
      console.error('Failed to preview the scenario draft', err)
      if (current === sequence) failed.value = true
    } finally {
      if (current === sequence) loading.value = false
    }
  }

  // A new scenario or a recomputed result starts again from its own values.
  watch(() => JSON.stringify(options.base()), reset)

  watch(
    () => [JSON.stringify(draft), blocked.value],
    () => {
      clearTimeout(timer)
      if (!dirty.value || blocked.value) {
        discard()
        return
      }
      if (!valid.value) return
      timer = setTimeout(refresh, debounceMs)
    },
  )

  onScopeDispose(() => {
    clearTimeout(timer)
    sequence += 1
  })

  return { draft, dirty, valid, blocked, preview, loading, failed, reset }
}
