<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import {
  movementsApi,
  KIND_LABELS,
  type Movement,
  type MovementKind,
  type MovementWrite,
  type MuscleInput,
  type MuscleRole,
  type Muscle,
} from '@/api/movements'
import { ApiError } from '@/api/client'

// When `movement` is supplied the modal edits it (PUT); otherwise it creates (POST).
const props = defineProps<{ movement?: Movement }>()
const emit = defineEmits<{ saved: [Movement]; close: [] }>()

const isEdit = computed(() => props.movement !== undefined)

// Flat form state. Tags/equipment are edited as comma-separated text and split on
// save; muscles are a structured list built with the picker below.
const form = reactive({
  name: props.movement?.name ?? '',
  movement_kind: (props.movement?.movement_kind ?? 'exercise') as MovementKind,
  favorite: props.movement?.favorite ?? false,
  measurable_rom: props.movement?.measurable_rom ?? false,
  rating: props.movement?.rating ?? null,
  tags: (props.movement?.tags ?? []).join(', '),
  equipment: (props.movement?.equipment ?? []).join(', '),
  how_to: props.movement?.how_to ?? '',
  form_cues: props.movement?.form_cues ?? '',
  common_faults: props.movement?.common_faults ?? '',
  default_sets: props.movement?.default_sets ?? null,
  default_reps: props.movement?.default_reps ?? '',
  default_hold_seconds: props.movement?.default_hold_seconds ?? null,
  sanskrit_name: props.movement?.sanskrit_name ?? '',
})

const muscles = reactive<MuscleInput[]>((props.movement?.muscles ?? []).map((m) => ({ muscle: m.muscle, role: m.role })))
const muscleOptions = ref<Muscle[]>([])
const pickMuscle = ref('')
const pickRole = ref<MuscleRole>('primary')

const showMore = ref(false)
const saving = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    muscleOptions.value = await movementsApi.muscles()
  } catch {
    // A failed muscle load just leaves the picker empty; the rest of the form works.
  }
})

function addMuscle() {
  if (!pickMuscle.value) return
  if (muscles.some((m) => m.muscle === pickMuscle.value && m.role === pickRole.value)) return
  muscles.push({ muscle: pickMuscle.value, role: pickRole.value })
  pickMuscle.value = ''
}

function removeMuscle(index: number) {
  muscles.splice(index, 1)
}

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

  const body: MovementWrite = {
    name: form.name.trim(),
    movement_kind: form.movement_kind,
    favorite: form.favorite,
    measurable_rom: form.measurable_rom,
    rating: form.rating || null,
    tags: splitList(form.tags),
    equipment: splitList(form.equipment),
    how_to: form.how_to,
    form_cues: form.form_cues,
    common_faults: form.common_faults,
    default_sets: form.default_sets || null,
    default_reps: form.default_reps.trim() || null,
    default_hold_seconds: form.default_hold_seconds || null,
    sanskrit_name: form.sanskrit_name.trim() || null,
    muscles: [...muscles],
  }

  try {
    const saved = props.movement ? await movementsApi.update(props.movement.id, body) : await movementsApi.create(body)
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
      :aria-label="isEdit ? 'Edit movement' : 'Add movement'">
      <header class="modal__head">
        <h2 class="modal__title">{{ isEdit ? 'Edit movement' : 'Add movement' }}</h2>
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
          <span class="field__label">Name</span>
          <input
            v-model="form.name"
            class="field__input"
            type="text"
            required />
        </label>

        <label class="field">
          <span class="field__label">Kind</span>
          <select
            v-model="form.movement_kind"
            class="field__input">
            <option
              v-for="(label, value) in KIND_LABELS"
              :key="value"
              :value="value">
              {{ label }}
            </option>
          </select>
        </label>

        <label class="field field--check">
          <input
            v-model="form.favorite"
            type="checkbox" />
          <span>Favorite</span>
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
            placeholder="mobility, posterior-chain" />
        </label>

        <label class="field">
          <span class="field__label">
            Equipment
            <em>(comma-separated)</em>
          </span>
          <input
            v-model="form.equipment"
            class="field__input"
            type="text"
            placeholder="barbell, mat" />
        </label>

        <fieldset class="field muscles">
          <legend class="field__label">Muscles</legend>
          <ul
            v-if="muscles.length"
            class="muscles__list">
            <li
              v-for="(m, i) in muscles"
              :key="`${m.muscle}-${m.role}`"
              class="muscles__item">
              {{ m.muscle }}
              <em>({{ m.role }})</em>
              <button
                type="button"
                class="muscles__remove"
                aria-label="Remove muscle"
                @click="removeMuscle(i)">
                ✕
              </button>
            </li>
          </ul>
          <div class="muscles__picker">
            <select
              v-model="pickMuscle"
              class="field__input"
              aria-label="Muscle">
              <option value="">Choose a muscle…</option>
              <option
                v-for="opt in muscleOptions"
                :key="opt.name"
                :value="opt.name">
                {{ opt.name }} ({{ opt.region }})
              </option>
            </select>
            <select
              v-model="pickRole"
              class="field__input muscles__role"
              aria-label="Role">
              <option value="primary">primary</option>
              <option value="secondary">secondary</option>
            </select>
            <button
              type="button"
              class="btn"
              @click="addMuscle">
              Add
            </button>
          </div>
        </fieldset>

        <label class="field">
          <span class="field__label">How to</span>
          <textarea
            v-model="form.how_to"
            class="field__input field__area"
            rows="3" />
        </label>

        <label class="field">
          <span class="field__label">Form cues</span>
          <textarea
            v-model="form.form_cues"
            class="field__input field__area"
            rows="2" />
        </label>

        <label class="field">
          <span class="field__label">Common faults</span>
          <textarea
            v-model="form.common_faults"
            class="field__input field__area"
            rows="2" />
        </label>

        <button
          type="button"
          class="more-toggle"
          @click="showMore = !showMore">
          {{ showMore ? '− Fewer fields' : '+ More fields' }}
        </button>

        <div
          v-if="showMore"
          class="more">
          <label class="field field--inline">
            <span class="field__label">Default sets</span>
            <input
              v-model.number="form.default_sets"
              class="field__input"
              type="number"
              min="0" />
          </label>
          <label class="field field--inline">
            <span class="field__label">Default reps</span>
            <input
              v-model="form.default_reps"
              class="field__input"
              type="text"
              placeholder="4–6, AMRAP, 30s" />
          </label>
          <label class="field field--inline">
            <span class="field__label">Default hold (s)</span>
            <input
              v-model.number="form.default_hold_seconds"
              class="field__input"
              type="number"
              min="0" />
          </label>
          <label class="field field--inline">
            <span class="field__label">Rating (1–5)</span>
            <input
              v-model.number="form.rating"
              class="field__input"
              type="number"
              min="1"
              max="5" />
          </label>
          <label class="field">
            <span class="field__label">Sanskrit name</span>
            <input
              v-model="form.sanskrit_name"
              class="field__input"
              type="text" />
          </label>
          <label class="field field--check">
            <input
              v-model="form.measurable_rom"
              type="checkbox" />
            <span>Track its range of motion as a measurement</span>
          </label>
        </div>

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

.muscles {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  margin: 0;
}

.muscles__list {
  list-style: none;
  margin: 0 0 var(--space-2);
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.muscles__item {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: 2px var(--space-2);
  border-radius: 999px;
  background: var(--surface-raised);
  font-size: 0.8rem;

  em {
    font-style: normal;
    color: var(--text-muted);
  }
}

.muscles__remove {
  border: none;
  background: transparent;
  color: var(--text-muted);
  padding: 0 2px;
}

.muscles__picker {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);

  .field__input {
    flex: 1 1 8rem;
  }
}

.muscles__role {
  flex: 0 0 auto;
}

.more-toggle {
  align-self: flex-start;
  border: none;
  background: transparent;
  color: var(--accent);
  padding: var(--space-1) 0;
}

.more {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding-top: var(--space-2);
  border-top: 1px dashed var(--border);
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
