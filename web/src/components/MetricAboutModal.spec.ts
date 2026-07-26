import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MetricAboutModal from './MetricAboutModal.vue'
import TrendChart from './TrendChart.vue'
import type { MetricTrend } from '@/api/measurements'

function trend(over: Partial<MetricTrend> = {}): MetricTrend {
  return {
    metric: 'heel-raise-capacity-right',
    label: 'Heel Raise Capacity Right',
    unit: 'reps',
    direction: 'higher_better',
    category: 'mobility',
    how_to_measure: 'Single-leg heel raises to failure on flat ground.\n\nKnee straight, full range.',
    points: [],
    first: null,
    latest: null,
    change: null,
    count: 0,
    ...over,
  }
}

describe('MetricAboutModal', () => {
  it('answers what the metric is and how to measure it', () => {
    const wrapper = mount(MetricAboutModal, { props: { trend: trend() } })

    expect(wrapper.text()).toContain('Heel Raise Capacity Right')
    expect(wrapper.text()).toContain('Single-leg heel raises to failure')
    expect(wrapper.text()).toContain('Knee straight, full range.')
    // The key is surfaced too — it is what `meso measurements record` addresses.
    expect(wrapper.text()).toContain('heel-raise-capacity-right')
    // Markdown, not raw text: the two blocks render as separate paragraphs.
    expect(wrapper.findAll('.prose p')).toHaveLength(2)
  })

  // An undocumented metric is the state the feedback was actually reporting, so the
  // modal has to name it rather than render an empty section that looks like a bug.
  it('says so when no protocol has been recorded', () => {
    const wrapper = mount(MetricAboutModal, { props: { trend: trend({ how_to_measure: '' }) } })

    expect(wrapper.find('.prose').exists()).toBe(false)
    expect(wrapper.text()).toContain('No protocol recorded yet')
  })

  it('offers edit and record without leaving the answer', async () => {
    const wrapper = mount(MetricAboutModal, { props: { trend: trend() } })

    await wrapper
      .findAll('button')
      .find((b) => b.text() === 'Edit')!
      .trigger('click')
    await wrapper
      .findAll('button')
      .find((b) => b.text() === 'Record')!
      .trigger('click')

    expect(wrapper.emitted('edit')).toHaveLength(1)
    expect(wrapper.emitted('record')).toHaveLength(1)
  })
})

// Tapping the name used to open the editor, which answers a question nobody asked.
describe('TrendChart', () => {
  it('opens the explanation from the metric name, not the editor', async () => {
    const wrapper = mount(TrendChart, { props: { trend: trend() } })

    await wrapper.find('.trend__name').trigger('click')

    expect(wrapper.emitted('about')).toHaveLength(1)
    expect(wrapper.emitted('edit')).toBeUndefined()
  })
})
