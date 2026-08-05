<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'

// The sheet every dialog sits in. Ten components had each hand-rolled this overlay,
// which meant ten chances for the safe-area padding or the escape handling to drift.
//
// It rises from the bottom on a phone and centres on a wide screen, because a sheet
// anchored to the bottom edge is the half of the screen a thumb can reach.
withDefaults(
  defineProps<{
    title: string
    /** Constrains the sheet for short dialogs so a two-line question isn't full-width. */
    narrow?: boolean
    /** Renders the body as a form, so Enter submits and the browser validates. */
    form?: boolean
  }>(),
  { narrow: false, form: false },
)
const emit = defineEmits<{ close: []; submit: [] }>()

// Escape closes. Handled here rather than per-dialog so it cannot be forgotten in one.
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}
onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div
    class="overlay"
    @click.self="emit('close')">
    <div
      class="modal"
      :class="{ 'modal--narrow': narrow }"
      role="dialog"
      aria-modal="true"
      :aria-label="title">
      <header class="modal__head">
        <h2 class="modal__title">{{ title }}</h2>
        <button
          class="modal__close"
          type="button"
          aria-label="Close"
          @click="emit('close')">
          ✕
        </button>
      </header>

      <form
        v-if="form"
        class="modal__body"
        @submit.prevent="emit('submit')">
        <slot />
      </form>
      <div
        v-else
        class="modal__body">
        <slot />
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
}

.modal {
  width: 100%;
  max-width: 40rem;
  max-height: 92dvh;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius) var(--radius) 0 0;

  &--narrow {
    max-width: 28rem;
  }
}

.modal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4);
  border-bottom: 1px solid var(--border);
}

.modal__title {
  margin: 0;
  font-size: 1.2rem;
}

.modal__close {
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 1.1rem;
}

.modal__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
  padding-bottom: calc(var(--space-4) + var(--safe-bottom));
  overflow-y: auto;
}

@media (min-width: 720px) {
  .overlay {
    align-items: center;
  }

  .modal {
    border-radius: var(--radius);
  }
}
</style>
