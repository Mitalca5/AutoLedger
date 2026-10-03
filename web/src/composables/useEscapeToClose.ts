import { getCurrentScope, onScopeDispose, watch } from 'vue'
import type { Ref } from 'vue'

// Escape closes the layer opened last: a confirmation over a modal, or a document preview over a form, does not
// close what is under it. Every modal, sheet and preview registers here while it is open.
const layers: Array<() => void> = []
let listening = false

export function closeTopLayerOnEscape(e: Pick<KeyboardEvent, 'key' | 'defaultPrevented' | 'isComposing' | 'preventDefault'>): boolean {
  if (e.key !== 'Escape' || e.defaultPrevented || e.isComposing) return false
  const close = layers[layers.length - 1]
  if (!close) return false
  e.preventDefault()
  close()
  return true
}

function listen() {
  if (listening || typeof window === 'undefined') return
  listening = true
  window.addEventListener('keydown', closeTopLayerOnEscape)
}

/** Registers a layer closed by Escape until the returned function is called. */
export function pushEscapeLayer(close: () => void): () => void {
  const layer = () => close()
  listen()
  layers.push(layer)
  return () => {
    const i = layers.indexOf(layer)
    if (i >= 0) layers.splice(i, 1)
  }
}

/** Closes the layer with `close` on Escape while `isOpen` is true (a ref or a getter), if no layer opened after it is still open. */
export function useEscapeToClose(isOpen: Ref<boolean> | (() => boolean), close: () => void) {
  const layer = () => close()
  const remove = () => {
    const i = layers.indexOf(layer)
    if (i >= 0) layers.splice(i, 1)
  }
  watch(
    isOpen,
    (open) => {
      if (open) {
        listen()
        if (!layers.includes(layer)) layers.push(layer)
      } else {
        remove()
      }
    },
    { immediate: true },
  )
  if (getCurrentScope()) onScopeDispose(remove)
}
