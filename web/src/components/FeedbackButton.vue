<script setup lang="ts">
import { ref } from 'vue'
import FeedbackModal from './FeedbackModal.vue'

// A persistent capture target on every route. It sits above the bottom tab bar on
// mobile — the one place a fixed control doesn't collide with either the tabs or the
// theme toggle — and stays semi-transparent until touched so it never competes with
// the content on a phone screen that is already tight.
const open = ref(false)
</script>

<template>
  <button
    class="fab"
    type="button"
    aria-label="Send feedback"
    @click="open = true">
    <span aria-hidden="true">✎</span>
  </button>

  <FeedbackModal
    v-if="open"
    @saved="open = false"
    @close="open = false" />
</template>

<style scoped lang="scss">
.fab {
  position: fixed;
  right: var(--space-3);
  // Clear the fixed tab bar and the iOS home indicator.
  bottom: calc(var(--touch-target) + var(--space-4) + var(--safe-bottom));
  z-index: 20;
  width: var(--touch-target);
  height: var(--touch-target);
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 1rem;
  opacity: 0.55;
  transition: opacity 120ms ease;

  &:hover,
  &:focus-visible,
  &:active {
    opacity: 1;
    color: var(--text);
  }
}

// No tab bar on desktop, so the button drops to the corner.
@media (min-width: 720px) {
  .fab {
    bottom: var(--space-4);
    right: var(--space-4);
  }
}
</style>
