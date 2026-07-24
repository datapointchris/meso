import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import WorkoutsView from './WorkoutsView.vue'
import type { Workout } from '@/api/workouts'

const listMock = vi.fn()
const updateMock = vi.fn()
vi.mock('@/api/workouts', async (importActual) => {
  const actual = await importActual<typeof import('@/api/workouts')>()
  return {
    ...actual,
    workoutsApi: {
      list: (...args: unknown[]) => listMock(...args),
      update: (...args: unknown[]) => updateMock(...args),
    },
  }
})

function fakeWorkout(over: Partial<Workout>): Workout {
  return {
    id: 1,
    name: 'Push Day',
    theme: 'push',
    tags: ['upper'],
    notes: '',
    favorite: false,
    estimated_minutes: null,
    movements: [],
    created_at: '',
    updated_at: '',
    ...over,
  }
}

async function mountView(): Promise<{ wrapper: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/workouts', name: 'workouts', component: WorkoutsView },
      { path: '/workouts/:id', name: 'workout-detail', component: { template: '<div />' } },
    ],
  })
  router.push('/workouts')
  await router.isReady()
  const wrapper = mount(WorkoutsView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('WorkoutsView', () => {
  beforeEach(() => {
    listMock.mockReset()
    updateMock.mockReset()
  })

  it('renders a card per workout with its movement count', async () => {
    listMock.mockResolvedValue([
      fakeWorkout({ id: 1, name: 'Push Day', movements: [] }),
      fakeWorkout({
        id: 2,
        name: 'Leg Day',
        theme: 'legs',
        movements: [
          {
            id: 9,
            movement_id: 3,
            movement_name: 'Squat',
            movement_kind: 'exercise',
            position: 1,
            sets: null,
            reps: null,
            load: null,
            rest_seconds: null,
            superset_group: null,
            notes: '',
          },
        ],
      }),
    ])
    const { wrapper } = await mountView()

    expect(wrapper.findAll('.card')).toHaveLength(2)
    expect(wrapper.text()).toContain('Push Day')
    expect(wrapper.text()).toContain('0 movements')
    expect(wrapper.text()).toContain('1 movement')
  })

  it('navigates to the detail route when a card is clicked', async () => {
    listMock.mockResolvedValue([fakeWorkout({ id: 7 })])
    const { wrapper, router } = await mountView()
    const push = vi.spyOn(router, 'push')

    await wrapper.find('.card').trigger('click')
    expect(push).toHaveBeenCalledWith({ name: 'workout-detail', params: { id: 7 } })
  })

  it('toggles favorite inline via the API without navigating', async () => {
    listMock.mockResolvedValue([fakeWorkout({ id: 3, favorite: false })])
    updateMock.mockResolvedValue(fakeWorkout({ id: 3, favorite: true }))
    const { wrapper, router } = await mountView()
    const push = vi.spyOn(router, 'push')

    await wrapper.find('.card__fav').trigger('click')
    expect(updateMock).toHaveBeenCalledWith(3, { favorite: true })
    expect(push).not.toHaveBeenCalled()
  })

  it('shows an error message when the API fails', async () => {
    listMock.mockRejectedValue(new Error('boom'))
    const { wrapper } = await mountView()
    expect(wrapper.find('.workouts__status--error').exists()).toBe(true)
  })
})
