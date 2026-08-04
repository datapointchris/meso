import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import ActiveSessionView from './ActiveSessionView.vue'
import type { Session, SessionMovement } from '@/api/sessions'
import type { Movement } from '@/api/movements'

const getMock = vi.fn()
const addMovementMock = vi.fn()
const removeMovementMock = vi.fn()
vi.mock('@/api/sessions', async (importActual) => {
  const actual = await importActual<typeof import('@/api/sessions')>()
  return {
    ...actual,
    sessionsApi: {
      get: (...args: unknown[]) => getMock(...args),
      addMovement: (...args: unknown[]) => addMovementMock(...args),
      removeMovement: (...args: unknown[]) => removeMovementMock(...args),
    },
  }
})

const movementListMock = vi.fn()
vi.mock('@/api/movements', async (importActual) => {
  const actual = await importActual<typeof import('@/api/movements')>()
  return { ...actual, movementsApi: { list: (...args: unknown[]) => movementListMock(...args) } }
})

function fakeEntry(over: Partial<SessionMovement> = {}): SessionMovement {
  return {
    id: 10,
    movement_id: 1,
    movement_name: 'Lat Pulldown',
    movement_kind: 'exercise',
    position: 1,
    done: false,
    actual_sets: null,
    actual_reps: null,
    actual_load: null,
    previous: null,
    notes: '',
    ...over,
  }
}

function fakeSession(over: Partial<Session> = {}): Session {
  return {
    id: 'abc',
    workout_id: null,
    workout_name: null,
    performed_on: '2026-08-04',
    duration_minutes: null,
    overall_notes: '',
    felt: null,
    movements: [],
    created_at: '',
    ...over,
  }
}

function fakeMovement(over: Partial<Movement> = {}): Movement {
  return {
    id: 1,
    name: 'Lat Pulldown',
    movement_kind: 'exercise',
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

async function mountView(): Promise<{ wrapper: ReturnType<typeof mount>; router: Router }> {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/sessions/:id', name: 'session-detail', component: ActiveSessionView },
      { path: '/sessions', name: 'sessions', component: { template: '<div />' } },
      { path: '/workouts/:id', name: 'workout-detail', component: { template: '<div />' } },
      { path: '/movements', name: 'movements', component: { template: '<div />' } },
    ],
  })
  router.push('/sessions/abc')
  await router.isReady()
  const wrapper = mount(ActiveSessionView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('ActiveSessionView — ad-hoc logging', () => {
  beforeEach(() => {
    getMock.mockReset()
    addMovementMock.mockReset()
    removeMovementMock.mockReset()
    movementListMock.mockReset()
    movementListMock.mockResolvedValue([fakeMovement()])
  })

  // The whole point of the ad-hoc path: an empty session is a starting point, not an
  // error state.
  it('invites the first movement instead of reporting an empty session', async () => {
    getMock.mockResolvedValue(fakeSession())
    const { wrapper } = await mountView()

    expect(wrapper.text()).toContain('Nothing logged yet')
    expect(wrapper.text()).toContain('Add a movement')
  })

  it('appends a picked movement and re-renders from the returned session', async () => {
    getMock.mockResolvedValue(fakeSession())
    addMovementMock.mockResolvedValue(fakeSession({ movements: [fakeEntry()] }))
    const { wrapper } = await mountView()

    await wrapper.find('.compose__open').trigger('click')
    await flushPromises()
    await wrapper.find('.picker__option').trigger('click')
    await flushPromises()

    expect(addMovementMock).toHaveBeenCalledWith('abc', { movement_id: 1 })
    expect(wrapper.text()).toContain('Lat Pulldown')
  })

  it('removes an entry added by mistake, after confirming', async () => {
    getMock.mockResolvedValue(fakeSession({ movements: [fakeEntry({ id: 22 })] }))
    removeMovementMock.mockResolvedValue(fakeSession())
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const { wrapper } = await mountView()

    await wrapper.find('.log-entry__remove').trigger('click')
    await flushPromises()

    expect(removeMovementMock).toHaveBeenCalledWith('abc', 22)
  })

  it('offers Save as workout only once something has been logged', async () => {
    getMock.mockResolvedValue(fakeSession())
    const { wrapper } = await mountView()
    expect(wrapper.text()).not.toContain('Save as workout')

    getMock.mockResolvedValue(fakeSession({ movements: [fakeEntry()] }))
    const { wrapper: logged } = await mountView()
    expect(logged.text()).toContain('Save as workout')
  })

  // A session copied from a template is edited by editing that workout; there is also
  // nothing to promote, because the template already exists.
  it('hides composing and promoting for a session backed by a workout', async () => {
    getMock.mockResolvedValue(fakeSession({ workout_id: 3, workout_name: 'Push Day', movements: [fakeEntry()] }))
    const { wrapper } = await mountView()

    expect(wrapper.find('.compose').exists()).toBe(false)
    expect(wrapper.find('.log-entry__remove').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Save as workout')
  })
})
