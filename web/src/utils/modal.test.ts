import { describe, expect, it } from 'vitest'
import { modalFooterClass, modalLayerClass, modalWidthClass } from './modal'

describe('modal classes', () => {
  it('gives each size its own width, growing with the size', () => {
    expect(modalWidthClass('sm')).toBe('max-w-md')
    expect(modalWidthClass('md')).toBe('max-w-xl')
    expect(modalWidthClass('lg')).toBe('max-w-3xl')
  })
  it('stacks a modal opened from another one above it', () => {
    expect(modalLayerClass(false)).toBe('z-modal')
    expect(modalLayerClass(true)).toBe('z-modal-nested')
  })
  it('keeps the footer bar and applies the alignment of the buttons', () => {
    expect(modalFooterClass('justify-end gap-2')).toContain('border-t border-slate-800/80')
    expect(modalFooterClass('justify-between')).toMatch(/justify-between$/)
  })
})
