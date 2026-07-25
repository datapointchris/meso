<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { logApi, type LogEntry, type LogEntryCreate, type LogEntryUpdate } from '@/api/log'
import { ApiError } from '@/api/client'

// When `entry` is supplied the modal edits it (PUT); otherwise it creates (POST).
const props = defineProps<{ entry?: LogEntry }>()
const emit = defineEmits<{ saved: [LogEntry]; close: [] }>()

const isEdit = computed(() => props.entry !== undefined)

// Flat form state. Tags are edited as comma-separated text and split on save.
const form = reactive({
  entry_date: props.entry?.entry_date ?? '',
  body: props.entry?.body ?? '',
  tags: (props.entry?.tags ?? []).join(', '),
  mood: props.entry?.mood ?? '',
})

const saving = ref(false)
const error = ref('')

function splitList(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

async function save() {
  if (!form.body.trim()) {
    error.value = 'Write something in the entry.'
    return
  }
  saving.value = true
  error.value = ''

  // mood is nullable — an empty field clears it to null rather than storing "".
  const mood = form.mood.trim() || null
  try {
    let saved: LogEntry
    if (props.entry) {
      const body: LogEntryUpdate = {
        entry_date: form.entry_date || undefined,
        body: form.body,
        tags: splitList(form.tags),
        mood,
      }
      saved = await logApi.update(props.entry.id, body)
    } else {
      const body: LogEntryCreate = {
        entry_date: form.entry_date || undefined,
        body: form.body,
        tags: splitList(form.tags),
        mood,
      }
      saved = await logApi.create(body)
    }
    emit('saved', saved)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save.'
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
      :aria-label="isEdit ? 'Edit log entry' : 'New log entry'">
      <header class="modal__head">
        <h2 class="modal__title">{{ isEdit ? 'Edit entry' : 'New entry' }}</h2>
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
        <label class="field">
          <span class="field__label">Entry (markdown)</span>
          <textarea
            v-model="form.body"
            class="field__input field__area"
            rows="8"
            placeholder="How did training feel? What stalled, what to carry forward?"
            required></textarea>
        </label>

        <div class="field-row">
          <label class="field">
            <span class="field__label">
              Date
              <em>(defaults to today)</em>
            </span>
            <input
              v-model="form.entry_date"
              class="field__input"
              type="date" />
          </label>
          <label class="field">
            <span class="field__label">Mood</span>
            <input
              v-model="form.mood"
              class="field__input"
              type="text"
              placeholder="strong, tired, focused…" />
          </label>
        </div>

        <label class="field">
          <span class="field__label">
            Tags
            <em>(comma-separated)</em>
          </span>
          <input
            v-model="form.tags"
            class="field__input"
            type="text"
            placeholder="strength, knee, pr" />
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
            {{ saving ? 'Saving…' : isEdit ? 'Save' : 'Add entry' }}
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
}

.modal {
  width: 100%;
  max-width: 32rem;
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

.field-row {
  display: flex;
  gap: var(--space-3);

  .field {
    flex: 1;
  }
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
  line-height: 1.5;
}

.modal__error {
  margin: 0;
  color: var(--negative);
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
