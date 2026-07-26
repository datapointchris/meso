<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { measurementsApi, formatValue, type Measurement } from '@/api/measurements'
import { ApiError } from '@/api/client'

// The readings behind one metric's trend, correctable in place. The trend card shows
// a shape; this is where a number that went in wrong gets fixed. Without it the API
// and CLI could edit a reading and the app could not, so a fat-fingered baseline
// meant dropping to a terminal.
const props = defineProps<{ metric: string; label: string; unit: string }>()
const emit = defineEmits<{ changed: []; close: [] }>()

const readings = ref<Measurement[]>([])
const loading = ref(true)
const error = ref('')

// The id being edited, plus the draft. One row edits at a time — a list of open
// forms on a phone is unusable.
const editingID = ref<number | null>(null)
const draft = reactive({ value: 0, measured_on: '', notes: '' })
const busy = ref(false)

async function load() {
  loading.value = true
  error.value = ''
  try {
    readings.value = await measurementsApi.list({ metric: props.metric })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load readings.'
  } finally {
    loading.value = false
  }
}

onMounted(load)

function startEdit(reading: Measurement) {
  editingID.value = reading.id
  draft.value = reading.value
  draft.measured_on = reading.measured_on
  draft.notes = reading.notes
}

async function save(id: number) {
  busy.value = true
  error.value = ''
  try {
    await measurementsApi.update(id, {
      value: draft.value,
      measured_on: draft.measured_on,
      notes: draft.notes,
    })
    editingID.value = null
    await load()
    emit('changed')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save.'
  } finally {
    busy.value = false
  }
}

async function remove(reading: Measurement) {
  if (!window.confirm(`Delete the ${formatValue(reading.value)} ${props.unit} reading from ${reading.measured_on}?`)) return
  busy.value = true
  error.value = ''
  try {
    await measurementsApi.remove(reading.id)
    await load()
    emit('changed')
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete.'
  } finally {
    busy.value = false
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
      :aria-label="`${label} readings`">
      <header class="modal__head">
        <h2 class="modal__title">{{ label }}</h2>
        <button
          class="modal__close"
          type="button"
          aria-label="Close"
          @click="emit('close')">
          ✕
        </button>
      </header>

      <div class="modal__body">
        <p
          v-if="loading"
          class="modal__status">
          Loading…
        </p>
        <p
          v-else-if="readings.length === 0"
          class="modal__status">
          No readings yet.
        </p>

        <ul
          v-else
          class="readings">
          <li
            v-for="reading in readings"
            :key="reading.id"
            class="reading">
            <template v-if="editingID === reading.id">
              <form
                class="edit"
                @submit.prevent="save(reading.id)">
                <label class="field">
                  <span class="field__label">Value</span>
                  <input
                    v-model.number="draft.value"
                    class="field__input"
                    type="number"
                    step="any"
                    inputmode="decimal"
                    required />
                </label>
                <label class="field">
                  <span class="field__label">Date</span>
                  <input
                    v-model="draft.measured_on"
                    class="field__input"
                    type="date"
                    required />
                </label>
                <label class="field">
                  <span class="field__label">Notes</span>
                  <input
                    v-model="draft.notes"
                    class="field__input"
                    type="text" />
                </label>
                <div class="edit__actions">
                  <button
                    type="button"
                    class="btn"
                    @click="editingID = null">
                    Cancel
                  </button>
                  <button
                    type="submit"
                    class="btn btn--accent"
                    :disabled="busy">
                    Save
                  </button>
                </div>
              </form>
            </template>

            <template v-else>
              <button
                class="reading__main"
                type="button"
                :aria-label="`Edit the reading from ${reading.measured_on}`"
                @click="startEdit(reading)">
                <span class="reading__value">
                  {{ formatValue(reading.value) }}
                  <em class="reading__unit">{{ unit }}</em>
                </span>
                <span class="reading__date">{{ reading.measured_on }}</span>
                <span
                  v-if="reading.notes"
                  class="reading__notes">
                  {{ reading.notes }}
                </span>
              </button>
              <button
                class="reading__delete"
                type="button"
                :aria-label="`Delete the reading from ${reading.measured_on}`"
                :disabled="busy"
                @click="remove(reading)">
                ✕
              </button>
            </template>
          </li>
        </ul>

        <p
          v-if="error"
          class="modal__error">
          {{ error }}
        </p>
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
  padding: var(--space-2) var(--space-4);
  padding-bottom: calc(var(--space-4) + var(--safe-bottom));
  overflow-y: auto;
}

.modal__status {
  padding: var(--space-4);
  color: var(--text-muted);
  text-align: center;
}

.modal__error {
  margin: var(--space-2) 0 0;
  color: var(--negative);
  font-size: 0.9rem;
}

.readings {
  margin: 0;
  padding: 0;
  list-style: none;
}

.reading {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  border-bottom: 1px solid var(--border);

  &:last-child {
    border-bottom: none;
  }
}

.reading__main {
  flex: 1;
  min-height: var(--touch-target);
  display: flex;
  align-items: baseline;
  gap: var(--space-3);
  padding: var(--space-2) 0;
  border: none;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
}

.reading__value {
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.reading__unit {
  font-style: normal;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--text-muted);
}

.reading__date {
  font-size: 0.8rem;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.reading__notes {
  flex: 1;
  font-size: 0.8rem;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reading__delete {
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: none;
  background: none;
  color: var(--text-muted);
  font-size: 0.9rem;

  &:disabled {
    opacity: 0.5;
  }
}

.edit {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-3) 0;
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.field__label {
  font-size: 0.85rem;
  color: var(--text-muted);
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

.edit__actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
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
