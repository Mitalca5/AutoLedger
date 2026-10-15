import { describe, expect, it } from 'vitest'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import EmptyState from './EmptyState.vue'

const render = (props: Record<string, unknown>, slots: Record<string, string> = {}) =>
  renderToString(createSSRApp({
    render: () => h(EmptyState, props as never, Object.fromEntries(Object.entries(slots).map(([k, v]) => [k, () => h('span', { class: `slot-${k}` }, v)]))),
  }))

describe('empty state', () => {
  it('shows the title, then the sentence, then the actions', async () => {
    const html = await render({ title: 'No charge recorded yet' }, { default: 'Charges show up here.', actions: 'Add a charge' })
    expect(html.indexOf('No charge recorded yet')).toBeLessThan(html.indexOf('Charges show up here.'))
    expect(html.indexOf('Charges show up here.')).toBeLessThan(html.indexOf('Add a charge'))
  })

  it('omits the sentence and the action block when there are none', async () => {
    const html = await render({ title: 'No reading yet' })
    expect(html).toContain('No reading yet')
    expect(html).not.toContain('max-w-md')
    expect(html).not.toContain('mt-4')
  })

  it('drops the icon and tightens the padding when compact', async () => {
    const Icon = { render: () => h('svg', { class: 'the-icon' }) }
    expect(await render({ title: 'T', icon: Icon })).toContain('the-icon')
    const compact = await render({ title: 'T', icon: Icon, compact: true })
    expect(compact).not.toContain('the-icon')
    expect(compact).toContain('py-6')
  })
})
