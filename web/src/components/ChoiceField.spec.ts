import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ChoiceField from './ChoiceField.vue'

const options = [
  { value: 'planned', label: 'planned' },
  { value: 'active', label: 'active' },
  { value: 'paused', label: 'paused' },
]

describe('ChoiceField', () => {
  it('presses the bound option and emits the value of the one tapped', async () => {
    const wrapper = mount(ChoiceField, { props: { modelValue: 'active', options, label: 'Status' } })
    const buttons = wrapper.findAll('.choicefield__option')

    expect(buttons.map((b) => b.attributes('aria-pressed'))).toEqual(['false', 'true', 'false'])

    await buttons[2].trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([['paused']])
  })
})
