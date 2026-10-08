export type ModalSize = 'sm' | 'md' | 'lg'

/** The three widths of a modal: a short form or a question, a form, a wide list or history. */
const WIDTHS: Record<ModalSize, string> = {
  sm: 'max-w-md',
  md: 'max-w-xl',
  lg: 'max-w-3xl',
}

export function modalWidthClass(size: ModalSize): string {
  return WIDTHS[size]
}

/** A modal opened from another one stacks above it (see the z-modal scale in main.css). */
export function modalLayerClass(nested: boolean): string {
  return nested ? 'z-modal-nested' : 'z-modal'
}

/** The footer bar every modal shares, with the alignment of its buttons. */
export function modalFooterClass(layout: string): string {
  return `px-5 py-3.5 border-t border-slate-800/80 flex shrink-0 bg-slate-900/95 ${layout}`.trim()
}
