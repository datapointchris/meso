import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import FeedbackModal from './FeedbackModal.vue'

const captureMock = vi.fn()
vi.mock('@/api/feedback', () => ({
  feedbackApi: { capture: (...args: unknown[]) => captureMock(...args) },
}))

async function mountModal(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/movements/:id', component: { template: '<div />' } }],
  })
  router.push(path)
  await router.isReady()
  return mount(FeedbackModal, { global: { plugins: [router] } })
}

describe('FeedbackModal', () => {
  beforeEach(() => {
    captureMock.mockReset()
    captureMock.mockResolvedValue({ id: '019f' })
  })

  // The route and the viewport are the whole triage payload: both are free at the
  // moment of the complaint and neither can be recovered from the body afterwards.
  it('captures the route and the viewport it was raised at', async () => {
    window.innerWidth = 390
    window.innerHeight = 844
    const wrapper = await mountModal('/movements/21')

    await wrapper.find('textarea').setValue('the how to is a big wall of text')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(captureMock).toHaveBeenCalledWith({
      body: 'the how to is a big wall of text',
      context_path: '/movements/21',
      viewport_width: 390,
      viewport_height: 844,
    })
  })

  it('refuses to capture an empty body without calling the API', async () => {
    const wrapper = await mountModal('/movements/21')

    await wrapper.find('textarea').setValue('   ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(captureMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Say what happened.')
  })
})
