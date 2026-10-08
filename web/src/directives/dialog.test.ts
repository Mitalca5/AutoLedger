import { describe, expect, it } from 'vitest'
import { protectUnsavedInput, shouldKeepOpen, wrapFocusIndex } from './dialog'

describe('wrapFocusIndex', () => {
  it('lets the browser move the focus inside the panel', () => {
    expect(wrapFocusIndex(3, 1, false)).toBe(-1)
    expect(wrapFocusIndex(3, 1, true)).toBe(-1)
  })
  it('wraps from the last control to the first and back', () => {
    expect(wrapFocusIndex(3, 2, false)).toBe(0)
    expect(wrapFocusIndex(3, 0, true)).toBe(2)
  })
  it('enters the panel when the focus is outside it', () => {
    expect(wrapFocusIndex(3, -1, false)).toBe(0)
    expect(wrapFocusIndex(3, -1, true)).toBe(2)
  })
  it('does nothing without a focusable control', () => {
    expect(wrapFocusIndex(0, -1, false)).toBe(-1)
  })
})

describe('shouldKeepOpen', () => {
  const backdrop = new EventTarget()
  const panelChild = new EventTarget()
  it('keeps a modal with edits open when the backdrop itself is clicked', () => {
    expect(shouldKeepOpen(true, backdrop, backdrop)).toBe(true)
  })
  it('lets a pristine modal close on a backdrop click', () => {
    expect(shouldKeepOpen(false, backdrop, backdrop)).toBe(false)
  })
  it('ignores clicks that do not land on the backdrop', () => {
    expect(shouldKeepOpen(true, panelChild, backdrop)).toBe(false)
    expect(shouldKeepOpen(true, null, null)).toBe(false)
  })
})

describe('protectUnsavedInput', () => {
  function setup() {
    const root = new EventTarget()
    const panel = new EventTarget()
    const backdrop = new EventTarget()
    let blocked = 0
    let reachedTheModal = 0
    const release = protectUnsavedInput(panel, backdrop, () => blocked++, root)
    // What runs after the guard on the way to the modal (its @click.self handler), registered later on the same root.
    root.addEventListener('click', () => reachedTheModal++)
    const click = (target: EventTarget) => {
      const event = new Event('click')
      Object.defineProperty(event, 'target', { value: target })
      root.dispatchEvent(event)
    }
    return { panel, backdrop, release, click, counts: () => ({ blocked, reachedTheModal }) }
  }

  it('lets a backdrop click through while nothing was edited', () => {
    const { backdrop, click, counts } = setup()
    click(backdrop)
    expect(counts()).toEqual({ blocked: 0, reachedTheModal: 1 })
  })

  it('stops the backdrop click and reports it after an input event', () => {
    const { panel, backdrop, click, counts } = setup()
    panel.dispatchEvent(new Event('input'))
    click(backdrop)
    expect(counts()).toEqual({ blocked: 1, reachedTheModal: 0 })
  })

  it('counts a change event as an edit too', () => {
    const { panel, backdrop, click, counts } = setup()
    panel.dispatchEvent(new Event('change'))
    click(backdrop)
    expect(counts()).toEqual({ blocked: 1, reachedTheModal: 0 })
  })

  it('ignores a click that lands elsewhere than on the backdrop itself', () => {
    const { panel, click, counts } = setup()
    panel.dispatchEvent(new Event('input'))
    click(new EventTarget())
    expect(counts()).toEqual({ blocked: 0, reachedTheModal: 1 })
  })

  it('stops guarding once released', () => {
    const { panel, backdrop, release, click, counts } = setup()
    panel.dispatchEvent(new Event('input'))
    release()
    click(backdrop)
    expect(counts()).toEqual({ blocked: 0, reachedTheModal: 1 })
  })

  it('never blocks a modal without a backdrop', () => {
    const root = new EventTarget()
    const panel = new EventTarget()
    let blocked = 0
    protectUnsavedInput(panel, null, () => blocked++, root)
    panel.dispatchEvent(new Event('input'))
    root.dispatchEvent(new Event('click'))
    expect(blocked).toBe(0)
  })
})
