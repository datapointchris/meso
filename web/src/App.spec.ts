import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import App from './App.vue'
import SessionsView from './views/SessionsView.vue'
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
        { path: '/', redirect: '/sessions' },
        { path: '/workouts', name: 'workouts', component: { template: '<div />' } },
        { path: '/sessions', name: 'sessions', component: SessionsView },
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

  // Sessions leads because starting or resuming training is why the app gets opened.
  it('renders all five primary tabs, sessions first', async () => {
    const { wrapper } = await mountApp()
    const labels = wrapper.findAll('.tabbar__label').map((n) => n.text())
    expect(labels).toEqual(['Sessions', 'Workouts', 'Movements', 'Log', 'Stats'])
  })

  it('lands on Sessions at /', async () => {
    const { wrapper } = await mountApp()
    expect(wrapper.text()).toContain('Start session')
  })
})
