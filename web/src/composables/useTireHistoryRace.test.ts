import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useTireHistory } from './useTireHistory'

const api = vi.hoisted(() => ({ getTireHistory: vi.fn() }))
vi.mock('@/services/api', () => ({ api }))
vi.mock('@/i18n', () => ({ t: (key: string) => key, intlLocale: () => 'en' }))
vi.mock('@/stores/vehicle', () => ({ useVehicleStore: () => ({ activeVehicle: { id: 'v' } }) }))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ showConfirm: vi.fn(), showAlert: vi.fn() }) }))

function history() {
  return useTireHistory({ tires: ref([]), loadTires: async () => {}, selectedTireIds: ref([]) })
}

function response(id: string) {
  return { tire: { id }, stats: {}, sessions: [], logs: [] }
}

describe('tire history request ordering', () => {
  it('ignores an older response after a newer selection and after closing', async () => {
    let finishFirst!: (value: any) => void
    let finishSecond!: (value: any) => void
    api.getTireHistory
      .mockImplementationOnce(() => new Promise(resolve => { finishFirst = resolve }))
      .mockImplementationOnce(() => new Promise(resolve => { finishSecond = resolve }))
    const h = history()
    const first = h.openHistoryModal({ tire: { id: 'A' } })
    const second = h.openHistoryModal({ tire: { id: 'B' } })
    finishSecond(response('B'))
    await second
    h.showHistoryModal.value = false
    finishFirst(response('A'))
    await first
    expect(h.selectedTire.value.id).toBe('B')
    expect(h.showHistoryModal.value).toBe(false)
  })

  it('does not reopen a dialog closed while its current history is refreshing', async () => {
    api.getTireHistory.mockResolvedValueOnce(response('A'))
    const h = history()
    await h.openHistoryModal({ tire: { id: 'A' } })
    let finish!: (value: any) => void
    api.getTireHistory.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const refresh = h.openHistoryModal({ tire: { id: 'A' } })
    h.showHistoryModal.value = false
    finish(response('A'))
    await refresh
    expect(h.showHistoryModal.value).toBe(false)
  })
})
