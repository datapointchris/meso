import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import ActiveSessionView from './ActiveSessionView.vue'
import type { Session, SessionMovement, SessionSet } from '@/api/sessions'
import type { Movement } from '@/api/movements'

const getMock = vi.fn()
const addMovementMock = vi.fn()
const removeMovementMock = vi.fn()
const addSetMock = vi.fn()
const finishMock = vi.fn()
vi.mock('@/api/sessions', async (importActual) => {
  const actual = await importActual<typeof import('@/api/sessions')>()
  return {
    ...actual,
    sessionsApi: {
      get: (...args: unknown[]) => getMock(...args),
      addMovement: (...args: unknown[]) => addMovementMock(...args),
      removeMovement: (...args: unknown[]) => removeMovementMock(...args),
      addSet: (...args: unknown[]) => addSetMock(...args),
      finish: (...args: unknown[]) => finishMock(...args),
    },
  }
})

const movementListMock = vi.fn()
vi.mock('@/api/movements', async (importActual) => {
  const actual = await importActual<typeof import('@/api/movements')>()
  return { ...actual, movementsApi: { list: (...args: unknown[]) => movementListMock(...args) } }
})

// The confirm dialog lives in the app shell, which isn't mounted here, so answer it
// directly rather than driving a component that isn't on screen.
const askMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ ask: askMock, request: { value: null }, answer: vi.fn() }),
}))

function fakeSet(over: Partial<SessionSet> = {}): SessionSet {
  return {
    id: 1,
    position: 1,
    reps: null,
    load: null,
    hold_seconds: null,
    set_kind: 'working',
    notes: '',
    logged_at: '2026-08-04T10:00:00Z',
    ...over,
  }
}

function fakeEntry(over: Partial<SessionMovement> = {}): SessionMovement {
  return {
    id: 10,
    movement_id: 1,
    movement_name: 'Lat Pulldown',
    movement_kind: 'exercise',
    load_mode: 'weighted',
    position: 1,
    done: false,
    target_sets: null,
    target_reps: null,
    target_load: null,
    sets: [],
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
    finished_at: null,
    ...over,
  }
}

function fakeMovement(over: Partial<Movement> = {}): Movement {
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

describe('ActiveSessionView — logging', () => {
  beforeEach(() => {
    getMock.mockReset()
    addMovementMock.mockReset()
    removeMovementMock.mockReset()
    addSetMock.mockReset()
    finishMock.mockReset()
    movementListMock.mockReset()
    askMock.mockReset()
    movementListMock.mockResolvedValue([fakeMovement()])
    askMock.mockResolvedValue(true)
  })

  // An empty session is a starting point, not an error state.
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

  // The screen's whole point: an empty body is what makes the server repeat the last set,
  // so the button must not send anything of its own.
  it('logs a set with no payload at all', async () => {
    getMock.mockResolvedValue(fakeSession({ movements: [fakeEntry()] }))
    addSetMock.mockResolvedValue(fakeSession({ movements: [fakeEntry({ sets: [fakeSet({ reps: 8, load: '100lb' })] })] }))
    const { wrapper } = await mountView()

    await wrapper.find('.log-set').trigger('click')
    await flushPromises()

    expect(addSetMock).toHaveBeenCalledWith('abc', 10, {})
    expect(wrapper.text()).toContain('8 reps · 100lb')
  })

  it('shows the plan as context and the last result to beat', async () => {
    getMock.mockResolvedValue(
      fakeSession({
        movements: [
          fakeEntry({
            target_sets: 3,
            target_reps: '8',
            target_load: '100lb',
            previous: { performed_on: '2026-07-28', sets: 3, reps: 8, load: '95lb' },
          }),
        ],
      }),
    )
    const { wrapper } = await mountView()

    expect(wrapper.text()).toContain('Plan: 3 × 8 · 100lb')
    expect(wrapper.text()).toContain('Last: 3 × 8 · 95lb · 2026-07-28')
  })

  it('removes an entry added by mistake, after confirming', async () => {
    getMock.mockResolvedValue(fakeSession({ movements: [fakeEntry({ id: 22 })] }))
    removeMovementMock.mockResolvedValue(fakeSession())
    const { wrapper } = await mountView()

    await wrapper.find('.entry__remove').trigger('click')
    await flushPromises()

    expect(askMock).toHaveBeenCalled()
    expect(removeMovementMock).toHaveBeenCalledWith('abc', 22)
  })

  // Reversed from the old rule: doing something the plan did not call for, or skipping
  // something it did, is part of what happened and has to be recordable.
  it('allows composing a session backed by a workout', async () => {
    getMock.mockResolvedValue(fakeSession({ workout_id: 3, workout_name: 'Push Day', movements: [fakeEntry()] }))
    const { wrapper } = await mountView()

    expect(wrapper.find('.compose').exists()).toBe(true)
    expect(wrapper.find('.entry__remove').exists()).toBe(true)
    // Promoting stays free-form only: a template-backed session would silently fork it.
    expect(wrapper.text()).not.toContain('Save as workout')
  })

  it('offers Save as workout only once a free-form session has something in it', async () => {
    getMock.mockResolvedValue(fakeSession())
    const { wrapper } = await mountView()
    expect(wrapper.text()).not.toContain('Save as workout')

    getMock.mockResolvedValue(fakeSession({ movements: [fakeEntry()] }))
    const { wrapper: logged } = await mountView()
    expect(logged.text()).toContain('Save as workout')
  })

  it('finishes the session and goes back to the list', async () => {
    getMock.mockResolvedValue(fakeSession({ movements: [fakeEntry()] }))
    finishMock.mockResolvedValue(fakeSession({ finished_at: '2026-08-04T11:00:00Z' }))
    const { wrapper, router } = await mountView()

    await wrapper.find('.finish-bar__btn').trigger('click')
    await flushPromises()

    expect(finishMock).toHaveBeenCalledWith('abc')
    expect(router.currentRoute.value.name).toBe('sessions')
  })

  // A finished session is history — there is nothing left to end.
  it('hides the finish bar once the session is over', async () => {
    getMock.mockResolvedValue(fakeSession({ finished_at: '2026-08-04T11:00:00Z', movements: [fakeEntry()] }))
    const { wrapper } = await mountView()

    expect(wrapper.find('.finish-bar__btn').exists()).toBe(false)
  })
})
