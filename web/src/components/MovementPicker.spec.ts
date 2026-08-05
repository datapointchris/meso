import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import MovementPicker from './MovementPicker.vue'
import type { Movement } from '@/api/movements'

const listMock = vi.fn()
vi.mock('@/api/movements', async (importActual) => {
  const actual = await importActual<typeof import('@/api/movements')>()
  return { ...actual, movementsApi: { list: (...args: unknown[]) => listMock(...args) } }
})

function fakeMovement(over: Partial<Movement>): Movement {
  return {
    id: 1,
    name: 'Lat Pulldown',
    movement_kind: 'exercise',
    load_mode: 'weighted',
    favorite: false,
    rating: null,
    tags: [],
    equipment: [],
    how_to: '',
    form_cues: '',
    common_faults: '',
    default_sets: null,
    default_reps: null,
    default_hold_seconds: null,
    sanskrit_name: null,
    measurable_rom: false,
    source_url: null,
    source_name: null,
    muscles: [],
    related: [],
    created_at: '',
    updated_at: '',
    ...over,
  }
}

async function mountPicker() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/movements', name: 'movements', component: { template: '<div />' } }],
  })
  router.push('/movements')
  await router.isReady()
  const wrapper = mount(MovementPicker, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

describe('MovementPicker', () => {
  beforeEach(() => {
    listMock.mockReset()
    vi.useRealTimers()
  })

  it('picks on a single tap, with no intermediate confirm step', async () => {
    listMock.mockResolvedValue([fakeMovement({ id: 4, name: 'Face Pull' })])
    const wrapper = await mountPicker()

    await wrapper.find('.picker__option').trigger('click')

    expect(wrapper.emitted('pick')).toHaveLength(1)
    expect((wrapper.emitted('pick')![0][0] as Movement).id).toBe(4)
  })

  // Matching is the API's job — sending the query is what lets punctuation and word
  // order differ from the library's spelling.
  it('sends the query to the server rather than filtering locally', async () => {
    listMock.mockResolvedValue([])
    const wrapper = await mountPicker()
    expect(listMock).toHaveBeenLastCalledWith({})

    vi.useFakeTimers()

    await wrapper.find('.picker__search').setValue('pull-down')
    vi.advanceTimersByTime(250)
    await flushPromises()

    expect(listMock).toHaveBeenLastCalledWith({ search: 'pull-down' })
  })

  it('names the miss and points at the library when nothing matches', async () => {
    listMock.mockResolvedValue([])
    const wrapper = await mountPicker()

    await wrapper.find('.picker__search').setValue('cable pull-down')
    await flushPromises()

    expect(wrapper.text()).toContain('cable pull-down')
    expect(wrapper.find('a').exists()).toBe(true)
  })

  it('blocks a second pick while the parent is still saving one', async () => {
    listMock.mockResolvedValue([fakeMovement({})])
    const wrapper = await mountPicker()
    await wrapper.setProps({ busy: true })

    await wrapper.find('.picker__option').trigger('click')

    expect(wrapper.emitted('pick')).toBeUndefined()
  })
})
