import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { useDriveSelection } from './useDriveSelection'

const d = (id: string, start: string) => ({ id, start_time: start })

describe('useDriveSelection', () => {
  it('toggles a drive on and off', () => {
    const drives = ref([d('a', '2026-01-02'), d('b', '2026-01-01')])
    const s = useDriveSelection(drives)
    s.toggleSelectDrive(drives.value[0])
    expect(s.selectedDriveIds.value).toEqual(['a'])
    s.toggleSelectDrive(drives.value[0])
    expect(s.selectedDriveIds.value).toEqual([])
  })

  it('lists the selection in chronological order', () => {
    const drives = ref([d('a', '2026-01-02'), d('b', '2026-01-01')])
    const s = useDriveSelection(drives)
    s.selectAll()
    expect(s.selectedList.value.map((x) => x.id)).toEqual(['b', 'a'])
    expect(s.allPageSelected.value).toBe(true)
  })

  it('reports partial selection and unselects the page on a second selectAll', () => {
    const drives = ref([d('a', '2026-01-02'), d('b', '2026-01-01')])
    const s = useDriveSelection(drives)
    s.toggleSelectDrive(drives.value[0])
    expect(s.somePageSelected.value).toBe(true)
    expect(s.allPageSelected.value).toBe(false)
    s.selectAll()
    expect(s.allPageSelected.value).toBe(true)
    s.selectAll()
    expect(s.selectedDriveIds.value).toEqual([])
  })

  it('keeps selections made on other pages and counts them off page', () => {
    const drives = ref([d('a', '2026-01-02')])
    const s = useDriveSelection(drives)
    s.selectAll()
    drives.value = [d('c', '2026-02-01')]
    expect(s.selectedOffPage.value).toBe(1)
    s.selectAll()
    expect(s.selectedDriveIds.value.sort()).toEqual(['a', 'c'])
    s.selectAll()
    expect(s.selectedDriveIds.value).toEqual(['a'])
    s.clearSelection()
    expect(s.selectedDriveIds.value).toEqual([])
  })

  it('is not "all selected" on an empty page', () => {
    const s = useDriveSelection(ref([]))
    expect(s.allPageSelected.value).toBe(false)
  })
})
