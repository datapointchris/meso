<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import { useConfirm } from '@/composables/useConfirm'
import FeedbackButton from '@/components/FeedbackButton.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const { theme, toggle } = useTheme()
// One dialog for the whole app, driven by useConfirm's singleton request.
const { request: confirmRequest, answer } = useConfirm()

// The five top surfaces. `to` matches the router names; the bottom-tab bar on
// mobile and the top bar on desktop both render from this one list.
//
// Sessions leads because it is what the app is opened to do: start training, or pick up
// the one already underway. Workouts stays a tab because picking a template is the other
// way in. Cycles gave up its slot — planning a block is a rare, considered act, so it
// lives one tap away in the Workouts header, where the Sessions link used to sit.
const tabs = [
  { to: '/sessions', label: 'Sessions', icon: '▶' },
  { to: '/workouts', label: 'Workouts', icon: '🏋' },
  { to: '/movements', label: 'Movements', icon: '📚' },
  { to: '/log', label: 'Log', icon: '📓' },
  { to: '/stats', label: 'Stats', icon: '📈' },
]
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <span class="brand">meso</span>
      <nav class="topbar__tabs">
        <RouterLink
          v-for="tab in tabs"
          :key="tab.to"
          :to="tab.to"
          class="topbar__tab">
          {{ tab.label }}
        </RouterLink>
      </nav>
      <button
        class="theme-toggle"
        type="button"
        :aria-label="`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`"
        @click="toggle">
        {{ theme === 'dark' ? '☀' : '☾' }}
      </button>
    </header>

    <main class="content">
      <RouterView />
    </main>

    <FeedbackButton />

    <ConfirmDialog
      v-if="confirmRequest"
      :title="confirmRequest.title"
      :message="confirmRequest.message"
      :confirm-label="confirmRequest.confirmLabel ?? 'Confirm'"
      :danger="confirmRequest.danger ?? false"
      @confirm="answer(true)"
      @cancel="answer(false)" />

    <nav
      class="tabbar"
      aria-label="Primary">
      <RouterLink
        v-for="tab in tabs"
        :key="tab.to"
        :to="tab.to"
        class="tabbar__tab">
        <span
          class="tabbar__icon"
          aria-hidden="true">
          {{ tab.icon }}
        </span>
        <span class="tabbar__label">{{ tab.label }}</span>
      </RouterLink>
    </nav>
  </div>
</template>

<style scoped lang="scss">
.app-shell {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}

// ─── Top bar: hidden on mobile (bottom tabs own nav there), shown ≥720px ──────
.topbar {
  display: none;
}

.content {
  flex: 1;
  width: 100%;
  max-width: var(--content-max);
  margin: 0 auto;
  padding: var(--space-4);
  // Leave room for the fixed bottom tab bar + iOS home indicator.
  padding-bottom: calc(var(--touch-target) + var(--space-6) + var(--safe-bottom));
}

// ─── Bottom tab bar (mobile-first, the default) ──────────────────────────────
.tabbar {
  position: fixed;
  inset: auto 0 0 0;
  display: flex;
  justify-content: space-around;
  background: var(--surface);
  border-top: 1px solid var(--border);
  padding-bottom: var(--safe-bottom);
  z-index: 10;
}

.tabbar__tab {
  flex: 1;
  min-height: var(--touch-target);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding: var(--space-2) 0;
  color: var(--text-muted);
  font-size: 0.7rem;

  &.router-link-active {
    color: var(--accent);
  }
}

.tabbar__icon {
  font-size: 1.15rem;
  line-height: 1;
}

// ─── Desktop: swap bottom tabs for a top bar ─────────────────────────────────
@media (min-width: 720px) {
  .tabbar {
    display: none;
  }

  .topbar {
    display: flex;
    align-items: center;
    gap: var(--space-6);
    padding: var(--space-3) var(--space-6);
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }

  .brand {
    font-weight: 700;
    font-size: 1.1rem;
    letter-spacing: 0.02em;
  }

  .topbar__tabs {
    display: flex;
    gap: var(--space-2);
    flex: 1;
  }

  .topbar__tab {
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    color: var(--text-muted);

    &.router-link-active {
      color: var(--text);
      background: var(--surface-raised);
    }
  }

  .content {
    padding-bottom: var(--space-8);
  }
}

.theme-toggle {
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  // On mobile, float the toggle at the top-right of the content instead of the
  // (hidden) top bar.
  position: fixed;
  top: calc(var(--safe-top) + var(--space-3));
  right: var(--space-3);
  z-index: 11;
}

@media (min-width: 720px) {
  .theme-toggle {
    position: static;
  }
}
</style>
