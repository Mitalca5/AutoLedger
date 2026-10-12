import { describe, expect, it } from 'vitest'
import { comparisonCurrency, canCompareCurrencies } from './comparisonCurrency'
import { buildComparisonPayload, scenarioToForm, emptyComparisonForm } from './comparisonForm'
const vehicles=[{id:'usd',currency:'USD'},{id:'eur',currency:'EUR'}]
describe('saved comparison currencies',()=>{
 it('resolves tracked currency without the active vehicle and keeps projections explicit',()=>{expect(comparisonCurrency({vehicle_id:'usd'},vehicles)).toBe('USD');expect(comparisonCurrency({mode:'PROJECTION',currency:'ARS'},vehicles)).toBe('ARS');expect(comparisonCurrency({mode:'PROJECTION'},vehicles)).toBeUndefined()})
 it('blocks cross-currency and unknown projections',()=>{expect(canCompareCurrencies([{vehicle_id:'usd'},{vehicle_id:'eur'}],vehicles)).toBe(false);expect(canCompareCurrencies([{currency:'USD'},{}],vehicles)).toBe(false);expect(canCompareCurrencies([{currency:'USD'},{vehicle_id:'usd'}],vehicles)).toBe(true)})
 it('round trips a projection currency including what-if copies',()=>{const f=emptyComparisonForm(false);f.currency='ARS';const payload=buildComparisonPayload(f,'eur');expect(payload.currency).toBe('ARS');expect(scenarioToForm(payload,false).currency).toBe('ARS');expect(scenarioToForm({...payload,currency:undefined},false).currency).toBe('')})
})
