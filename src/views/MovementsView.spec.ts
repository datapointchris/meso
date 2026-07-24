import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import MovementsView from './MovementsView.vue'
import type { Movement } from '@/api/movements'

// Mock the API module, keeping the real helpers/labels but stubbing the network.
const listMock = vi.fn()
const updateMock = vi.fn()
vi.mock('@/api/movements', async (importActual) => {
  const actual = await importActual<typeof import('@/api/movements')>()
  return {
    ...actual,
    movementsApi: {
      list: (...args: unknown[]) => listMock(...args),
      update: (...args: unknown[]) => updateMock(...args),
      muscles: vi.fn().mockResolvedValue([]),
    },
  }
})

function fakeMovement(over: Partial<Movement>): Movement {
  return {
    id: 1,
    name: 'Barbell Deadlift',
    movement_kind: 'exercise',
    favorite: false,
    rating: null,
    tags: ['strength', 'posterior-chain'],
    equipment: ['barbell'],
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
    muscles: [{ muscle: 'hamstrings', region: 'posterior', role: 'primary' }],
    related: [],
    created_at: '',
    updated_at: '',
    ...over,
  }
}

async function mountView(): Promise<{ wrapper: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/movements', name: 'movements', component: MovementsView },
      { path: '/movements/:id', name: 'movement-detail', component: { template: '<div />' } },
    ],
  })
  router.push('/movements')
  await router.isReady()
  const wrapper = mount(MovementsView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('MovementsView', () => {
  beforeEach(() => {
    listMock.mockReset()
    updateMock.mockReset()
  })

  it('renders a card per movement returned by the API', async () => {
    listMock.mockResolvedValue([
      fakeMovement({ id: 1, name: 'Barbell Deadlift' }),
      fakeMovement({ id: 2, name: "Child's Pose", movement_kind: 'yoga_pose' }),
    ])
    const { wrapper } = await mountView()

    const cards = wrapper.findAll('.card')
    expect(cards).toHaveLength(2)
    expect(wrapper.text()).toContain('Barbell Deadlift')
    expect(wrapper.text()).toContain("Child's Pose")
    // Primary muscle surfaced on the card.
    expect(wrapper.text()).toContain('hamstrings')
  })

  it('navigates to the detail route when a card is clicked', async () => {
    listMock.mockResolvedValue([fakeMovement({ id: 7 })])
    const { wrapper, router } = await mountView()
    const push = vi.spyOn(router, 'push')

    await wrapper.find('.card').trigger('click')
    expect(push).toHaveBeenCalledWith({ name: 'movement-detail', params: { id: 7 } })
  })

  it('toggles favorite inline via the API without navigating', async () => {
    listMock.mockResolvedValue([fakeMovement({ id: 3, favorite: false })])
    updateMock.mockResolvedValue(fakeMovement({ id: 3, favorite: true }))
    const { wrapper, router } = await mountView()
    const push = vi.spyOn(router, 'push')

    await wrapper.find('.card__fav').trigger('click')
    expect(updateMock).toHaveBeenCalledWith(3, { favorite: true })
    // Clicking the star must not open the detail view.
    expect(push).not.toHaveBeenCalled()
  })

  it('shows an error message when the API fails', async () => {
    listMock.mockRejectedValue(new Error('boom'))
    const { wrapper } = await mountView()
    expect(wrapper.find('.movements__status--error').exists()).toBe(true)
  })
})
