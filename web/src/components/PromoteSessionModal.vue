<script setup lang="ts">
import { reactive, ref } from 'vue'
import { sessionsApi, type SessionPromote } from '@/api/sessions'
import type { Workout } from '@/api/workouts'
import { ApiError } from '@/api/client'

// Turn a session that was made up on the spot into a workout worth repeating. The
// movements and their numbers come from what was actually logged, so all that is asked
// for here is how to label it.
const props = defineProps<{ sessionId: string; movementCount: number }>()
const emit = defineEmits<{ promoted: [Workout]; close: [] }>()

const form = reactive({ name: '', theme: '', tags: '', notes: '' })
const saving = ref(false)
const error = ref('')

async function save() {
  if (!form.name.trim()) {
    error.value = 'Name is required.'
    return
  }
  saving.value = true
  error.value = ''

  const body: SessionPromote = {
    name: form.name.trim(),
    theme: form.theme.trim() || null,
    tags: form.tags
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean),
    notes: form.notes,
  }

  try {
    emit('promoted', await sessionsApi.promote(props.sessionId, body))
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save as a workout.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div
    class="overlay"
    @click.self="emit('close')">
    <div
      class="modal"
      role="dialog"
      aria-modal="true"
      aria-label="Save session as workout">
      <header class="modal__head">
        <h2 class="modal__title">Save as workout</h2>
        <button
          class="modal__close"
          type="button"
          aria-label="Close"
          @click="emit('close')">
          ✕
        </button>
      </header>

      <form
        class="modal__body"
        @submit.prevent="save">
        <p class="modal__lede">
          {{ props.movementCount }} logged {{ props.movementCount === 1 ? 'movement' : 'movements' }} become the prescription, in the order
          you did them.
        </p>

        <label class="field">
          <span class="field__label">Name</span>
          <input
            v-model="form.name"
            class="field__input"
            type="text"
            placeholder="Cable pull day"
            required />
        </label>

        <label class="field">
          <span class="field__label">Theme</span>
          <input
            v-model="form.theme"
            class="field__input"
            type="text"
            placeholder="pull, lower + shoulder rehab" />
        </label>

        <label class="field">
          <span class="field__label">
            Tags
            <em>(comma-separated)</em>
          </span>
          <input
            v-model="form.tags"
            class="field__input"
            type="text"
            placeholder="upper, strength" />
        </label>

        <label class="field">
          <span class="field__label">Notes</span>
          <textarea
            v-model="form.notes"
            class="field__input field__area"
            rows="3" />
        </label>

        <p
          v-if="error"
          class="modal__error">
          {{ error }}
        </p>

        <div class="modal__actions">
          <button
            type="button"
            class="btn"
            @click="emit('close')">
            Cancel
          </button>
          <button
            type="submit"
            class="btn btn--accent"
            :disabled="saving">
            {{ saving ? 'Saving…' : 'Save workout' }}
          </button>
        </div>
      </form>
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
  padding: 0;
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

.modal__lede {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.field__label {
  font-size: 0.85rem;
  color: var(--text-muted);

  em {
    font-style: normal;
    opacity: 0.7;
  }
}

.field__input {
  min-height: var(--touch-target);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
}

.field__area {
  min-height: auto;
  resize: vertical;
}

.modal__error {
  margin: 0;
  color: #f87171;
  font-size: 0.9rem;
}

.modal__actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  padding-top: var(--space-2);
}

.btn {
  min-height: var(--touch-target);
  padding: 0 var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface-raised);
  color: var(--text);
  font-weight: 600;

  &--accent {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }

  &:disabled {
    opacity: 0.6;
  }
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
