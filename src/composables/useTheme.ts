import { ref } from 'vue'

// Theme is a design-variable switch: it sets data-theme on <html>, and all
// colors resolve from CSS custom properties keyed off that attribute (see
// styles/main.scss). This is the seam the design-style switcher will extend —
// palettes stay in CSS, never hardcoded in components.
export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'meso-theme'

// Module-level singleton so every caller shares one reactive theme.
const theme = ref<Theme>('dark')

function apply(next: Theme): void {
  theme.value = next
  document.documentElement.setAttribute('data-theme', next)
}

function preferredTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'light' || stored === 'dark') return stored
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

export function useTheme() {
  function init(): void {
    apply(preferredTheme())
  }

  function toggle(): void {
    const next: Theme = theme.value === 'dark' ? 'light' : 'dark'
    localStorage.setItem(STORAGE_KEY, next)
    apply(next)
  }

  return { theme, init, toggle }
}
