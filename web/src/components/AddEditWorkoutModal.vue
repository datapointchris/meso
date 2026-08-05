<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { workoutsApi, type Workout, type WorkoutWrite } from '@/api/workouts'
import { ApiError } from '@/api/client'
import ModalShell from './ModalShell.vue'

// When `workout` is supplied the modal edits it (PUT); otherwise it creates (POST).
const props = defineProps<{ workout?: Workout }>()
const emit = defineEmits<{ saved: [Workout]; close: [] }>()

const isEdit = computed(() => props.workout !== undefined)

const form = reactive({
  name: props.workout?.name ?? '',
  theme: props.workout?.theme ?? '',
  tags: (props.workout?.tags ?? []).join(', '),
  notes: props.workout?.notes ?? '',
  favorite: props.workout?.favorite ?? false,
  estimated_minutes: props.workout?.estimated_minutes ?? null,
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
  if (!form.name.trim()) {
    error.value = 'Name is required.'
    return
  }
  saving.value = true
  error.value = ''

  const body: WorkoutWrite = {
    name: form.name.trim(),
    theme: form.theme.trim() || null,
    tags: splitList(form.tags),
    notes: form.notes,
    favorite: form.favorite,
    estimated_minutes: form.estimated_minutes || null,
  }

  try {
    const saved = props.workout ? await workoutsApi.update(props.workout.id, body) : await workoutsApi.create(body)
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
    :title="isEdit ? 'Edit workout' : 'Add workout'"
    form
    @submit="save"
    @close="emit('close')">
    <label class="field">
      <span class="field__label">Name</span>
      <input
        v-model="form.name"
        class="field__input"
        type="text"
        required />
    </label>

    <label class="field">
      <span class="field__label">Theme</span>
      <input
        v-model="form.theme"
        class="field__input"
        type="text"
        placeholder="push, lower + shoulder rehab" />
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

    <label class="field field--check">
      <input
        v-model="form.favorite"
        type="checkbox" />
      <span>Favorite</span>
    </label>

    <label class="field field--inline">
      <span class="field__label">Estimated minutes</span>
      <input
        v-model.number="form.estimated_minutes"
        class="field__input"
        type="number"
        min="0" />
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
        {{ saving ? 'Saving…' : isEdit ? 'Save' : 'Create' }}
      </button>
    </div>
  </ModalShell>
</template>

<style scoped lang="scss">
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

.field--check {
  flex-direction: row;
  align-items: center;
  gap: var(--space-2);
}

.field--inline {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);

  .field__input {
    max-width: 12rem;
  }
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
</style>
