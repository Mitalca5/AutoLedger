import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useTireHistory } from './useTireHistory'
const api=vi.hoisted(()=>({getTireHistory:vi.fn()}))
vi.mock('@/services/api',()=>({api}))
vi.mock('@/i18n',()=>({t:(k:string)=>k,intlLocale:()=> 'en'}))
vi.mock('@/stores/vehicle',()=>({useVehicleStore:()=>({activeVehicle:{id:'v'}})}))
vi.mock('@/composables/useConfirm',()=>({useConfirm:()=>({showConfirm:vi.fn(),showAlert:vi.fn()})}))
describe('tire history request ordering',()=>{
 it('ignores an older response after a newer selection and after closing',async()=>{
  let first!:(v:any)=>void;let second!:(v:any)=>void
  api.getTireHistory.mockImplementationOnce(()=>new Promise(r=>{first=r})).mockImplementationOnce(()=>new Promise(r=>{second=r}))
  const h=useTireHistory({tires:ref([]),loadTires:async()=>{},selectedTireIds:ref([])})
  const a=h.openHistoryModal({tire:{id:'A'}});const b=h.openHistoryModal({tire:{id:'B'}})
  second({tire:{id:'B'},stats:{},sessions:[],logs:[]});await b
  h.showHistoryModal.value=false
  first({tire:{id:'A'},stats:{},sessions:[],logs:[]});await a
  expect(h.selectedTire.value.id).toBe('B');expect(h.showHistoryModal.value).toBe(false)
 })
 it('does not reopen a dialog closed while its current history is refreshing',async()=>{
  api.getTireHistory.mockResolvedValueOnce({tire:{id:'A'},stats:{},sessions:[],logs:[]})
  const h=useTireHistory({tires:ref([]),loadTires:async()=>{},selectedTireIds:ref([])})
  await h.openHistoryModal({tire:{id:'A'}})
  let finish!:(v:any)=>void
  api.getTireHistory.mockImplementationOnce(()=>new Promise(r=>{finish=r}))
  const refresh=h.openHistoryModal({tire:{id:'A'}})
  h.showHistoryModal.value=false
  finish({tire:{id:'A'},stats:{},sessions:[],logs:[]});await refresh
  expect(h.showHistoryModal.value).toBe(false)
 })
})
