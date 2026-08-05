<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { logApi, type LogEntry, type LogEntryCreate, type LogEntryUpdate } from '@/api/log'
import { ApiError } from '@/api/client'
import DateField from '@/components/DateField.vue'
import ModalShell from './ModalShell.vue'

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
  <ModalShell
    :title="isEdit ? 'Edit entry' : 'New entry'"
    form
    @submit="save"
    @close="emit('close')">
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
      <div class="field">
        <span class="field__label">
          Date
          <em>(defaults to today)</em>
        </span>
        <DateField
          v-model="form.entry_date"
          label="Entry date"
          clearable />
      </div>
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
  </ModalShell>
</template>

<style scoped lang="scss">
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
</style>
