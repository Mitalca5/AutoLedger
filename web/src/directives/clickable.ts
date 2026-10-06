import type { Directive } from 'vue'

// v-clickable goes on a container that opens something when clicked (a card, a row): it adds role="button" and a tab stop,
// and Enter or Space on the container itself triggers the same click. Keys pressed on a control inside it (a checkbox,
// an edit button) keep their own behaviour.
export function isActivationKey(key: string): boolean {
  return key === 'Enter' || key === ' ' || key === 'Spacebar'
}

const handlers = new WeakMap<HTMLElement, (e: KeyboardEvent) => void>()

export const vClickable: Directive<HTMLElement> = {
  mounted(el) {
    if (!el.hasAttribute('role')) el.setAttribute('role', 'button')
    if (!el.hasAttribute('tabindex')) el.setAttribute('tabindex', '0')
    const onKeydown = (e: KeyboardEvent) => {
      if (e.target !== el || e.defaultPrevented || !isActivationKey(e.key)) return
      e.preventDefault()
      el.click()
    }
    el.addEventListener('keydown', onKeydown)
    handlers.set(el, onKeydown)
  },
  unmounted(el) {
    const onKeydown = handlers.get(el)
    if (onKeydown) el.removeEventListener('keydown', onKeydown)
    handlers.delete(el)
  },
}
