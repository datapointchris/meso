import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DateField from './DateField.vue'

async function openOn(value: string) {
  const wrapper = mount(DateField, { props: { modelValue: value, clearable: true } })
  await wrapper.find('.datefield__trigger').trigger('click')
  return wrapper
}

function dayButton(wrapper: ReturnType<typeof mount>, day: string) {
  return wrapper.findAll('.datefield__day').find((b) => b.text() === day)!
}

describe('DateField', () => {
  it('shows the bound date and opens on the month it is in', async () => {
    const wrapper = await openOn('2026-08-04')

    expect(wrapper.find('.datefield__trigger').text()).toContain('Aug 4, 2026')
    expect(wrapper.find('.datefield__month').text()).toBe('August 2026')
    expect(dayButton(wrapper, '4').classes()).toContain('datefield__day--selected')
  })

  it('emits an ISO date when a day is picked, and closes', async () => {
    const wrapper = await openOn('2026-08-04')

    await dayButton(wrapper, '19').trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['2026-08-19'])
    expect(wrapper.find('.datefield__popover').exists()).toBe(false)
  })

  // A date is a calendar date, not an instant: the string that goes back must be the
  // day that was tapped regardless of the browser's timezone.
  it('round-trips the tapped day without timezone drift', async () => {
    const wrapper = await openOn('2026-01-15')

    await dayButton(wrapper, '1').trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['2026-01-01'])
  })

  it('navigates months across a year boundary', async () => {
    const wrapper = await openOn('2026-01-15')

    await wrapper.find('[aria-label="Previous month"]').trigger('click')
    expect(wrapper.find('.datefield__month').text()).toBe('December 2025')

    await wrapper.find('[aria-label="Next month"]').trigger('click')
    await wrapper.find('[aria-label="Next month"]').trigger('click')
    expect(wrapper.find('.datefield__month').text()).toBe('February 2026')
  })

  it('walks days with the arrow keys, pulling the month along', async () => {
    const wrapper = await openOn('2026-03-01')
    const popover = wrapper.find('.datefield__popover')

    await popover.trigger('keydown', { key: 'ArrowLeft' })
    expect(wrapper.find('.datefield__month').text()).toBe('February 2026')

    await popover.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('update:modelValue')![0]).toEqual(['2026-02-28'])
  })

  it('closes on Escape without changing the value', async () => {
    const wrapper = await openOn('2026-08-04')

    await wrapper.find('.datefield__popover').trigger('keydown', { key: 'Escape' })

    expect(wrapper.find('.datefield__popover').exists()).toBe(false)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  // The date filters are optional, so an empty value has to be both reachable and
  // legible rather than showing a stale date.
  it('reads as unset when empty, and clears back to unset', async () => {
    const wrapper = await openOn('')

    expect(wrapper.find('.datefield__placeholder').exists()).toBe(true)
    await wrapper
      .findAll('.datefield__action')
      .find((b) => b.text() === 'Clear')!
      .trigger('click')

    expect(wrapper.emitted('update:modelValue')![0]).toEqual([''])
  })

  it('hides Clear where a date is required', async () => {
    const wrapper = mount(DateField, { props: { modelValue: '2026-08-04' } })
    await wrapper.find('.datefield__trigger').trigger('click')

    expect(wrapper.findAll('.datefield__action').map((b) => b.text())).toEqual(['Today'])
  })
})
