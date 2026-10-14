import { onMounted, onUnmounted, ref } from 'vue'

// True below the `sm` breakpoint (640px), kept in step with the window. Used where a phone gets another
// container (a sheet) rather than the same markup squeezed in.
export function useIsMobile() {
  const query = typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia('(max-width: 639px)') : null
  const isMobile = ref(query?.matches ?? false)
  const update = () => {
    isMobile.value = query?.matches ?? false
  }
  onMounted(() => query?.addEventListener('change', update))
  onUnmounted(() => query?.removeEventListener('change', update))
  return isMobile
}
