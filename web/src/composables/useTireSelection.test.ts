import { computed, ref } from 'vue'
import { describe, expect, it } from 'vitest'
import { useTireSelection } from './useTireSelection'

const tire = (id: string, position: string) => ({ tire: { id, current_position: position } })

function setup(tab: 'chassis' | 'storage' | 'disposed' = 'chassis') {
  const tires = ref([tire('a', 'FL'), tire('b', 'RR'), tire('c', 'STORAGE'), tire('d', 'DISPOSED')])
  const mountedTires = computed(() => {
    const map: Record<string, any> = { FL: null, FR: null, RL: null, RR: null }
    for (const t of tires.value) if (t.tire.current_position in map) map[t.tire.current_position] = t
    return map
  })
  const storageTires = computed(() => tires.value.filter((t) => t.tire.current_position === 'STORAGE'))
  const disposedTires = computed(() => tires.value.filter((t) => t.tire.current_position === 'DISPOSED'))
  const activeTab = ref(tab)
  return { activeTab, sel: useTireSelection({ tires, mountedTires, storageTires, disposedTires, activeTab }) }
}

describe('useTireSelection', () => {
  it('lists the tire ids of the tab on display', () => {
    const { activeTab, sel } = setup()
    expect(sel.currentTabTireIds.value).toEqual(['a', 'b'])
    activeTab.value = 'storage'
    expect(sel.currentTabTireIds.value).toEqual(['c'])
    activeTab.value = 'disposed'
    expect(sel.currentTabTireIds.value).toEqual(['d'])
  })

  it('toggles one tire and reports a partial selection', () => {
    const { sel } = setup()
    sel.toggleTireSelection('a')
    expect(sel.selectedTireIds.value).toEqual(['a'])
    expect(sel.isCurrentTabPartlySelected.value).toBe(true)
    expect(sel.isCurrentTabAllSelected.value).toBe(false)
    sel.toggleTireSelection('a')
    expect(sel.selectedTireIds.value).toEqual([])
  })

  it('selects the whole tab, keeping other tabs, then unselects only that tab', () => {
    const { activeTab, sel } = setup()
    sel.toggleTireSelection('c')
    sel.toggleSelectAllCurrentTab()
    expect(sel.selectedTireIds.value.sort()).toEqual(['a', 'b', 'c'])
    expect(sel.isCurrentTabAllSelected.value).toBe(true)
    sel.toggleSelectAllCurrentTab()
    expect(sel.selectedTireIds.value).toEqual(['c'])
    activeTab.value = 'storage'
    expect(sel.isCurrentTabAllSelected.value).toBe(true)
  })

  it('does nothing on an empty tab', () => {
    const { sel } = setup()
    const empty = useTireSelection({
      tires: ref([]),
      mountedTires: computed(() => ({})),
      storageTires: computed(() => []),
      disposedTires: computed(() => []),
      activeTab: ref('storage'),
    })
    empty.toggleSelectAllCurrentTab()
    expect(empty.selectedTireIds.value).toEqual([])
    expect(empty.isCurrentTabAllSelected.value).toBe(false)
    expect(sel.canBatchDispose.value).toBe(false)
  })

  it('allows a batch dispose only when a selected tire is not already disposed', () => {
    const { sel } = setup()
    sel.toggleTireSelection('d')
    expect(sel.canBatchDispose.value).toBe(false)
    sel.toggleTireSelection('a')
    expect(sel.canBatchDispose.value).toBe(true)
  })
})
