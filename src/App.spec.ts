import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import App from './App.vue'
import WorkoutsView from './views/WorkoutsView.vue'
import MovementsView from './views/MovementsView.vue'

describe('App shell', () => {
  beforeEach(() => {
    // jsdom has no matchMedia; useTheme reads it on first render.
    window.matchMedia = ((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia
  })

  async function mountApp() {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', redirect: '/workouts' },
        { path: '/workouts', name: 'workouts', component: WorkoutsView },
        { path: '/sessions', name: 'sessions', component: { template: '<div />' } },
        { path: '/movements', name: 'movements', component: MovementsView },
        { path: '/cycles', name: 'cycles', component: { template: '<div />' } },
        { path: '/log', name: 'log', component: { template: '<div />' } },
        { path: '/stats', name: 'stats', component: { template: '<div />' } },
      ],
    })
    router.push('/')
    await router.isReady()
    const wrapper = mount(App, { global: { plugins: [router] } })
    return { wrapper, router }
  }

  it('renders all five primary tabs', async () => {
    const { wrapper } = await mountApp()
    const labels = wrapper.findAll('.tabbar__label').map((n) => n.text())
    expect(labels).toEqual(['Workouts', 'Movements', 'Cycles', 'Log', 'Stats'])
  })

  it('lands on the Workouts view at /', async () => {
    const { wrapper } = await mountApp()
    expect(wrapper.text()).toContain('Workouts')
  })
})
