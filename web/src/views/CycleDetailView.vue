<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  cyclesApi,
  cyclePrescriptionSummary,
  cycleTargetSummary,
  type Cycle,
  type CycleWorkout,
  type CycleWorkoutPatch,
} from '@/api/cycles'
import { workoutsApi, type Workout } from '@/api/workouts'
import { ApiError } from '@/api/client'
import { renderMarkdown } from '@/composables/useMarkdown'
import AddEditCycleModal from '@/components/AddEditCycleModal.vue'

const route = useRoute()
const router = useRouter()

const cycle = ref<Cycle | null>(null)
const loading = ref(true)
const error = ref('')
const showEdit = ref(false)

const id = computed(() => Number(route.params.id))

// The workout library, loaded once to populate the "add workout" and swap pickers.
const library = ref<Workout[]>([])
const pickWorkout = ref<number | ''>('')
const addForm = reactive({ week: null as number | null, phase: '', frequency: '', intensity: '', conditions: '' })
const librarySearch = ref('')

// Inline periodization editing: which entry is open, its edit buffer, and an optional
// swap target (workouts have no relationships, so any workout is a valid substitute).
const editingEntry = ref<number | null>(null)
const editForm = reactive({
  workout: '' as number | '',
  week: null as number | null,
  phase: '',
  frequency: '',
  intensity: '',
  conditions: '',
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    cycle.value = await cyclesApi.get(id.value)
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      error.value = 'That cycle no longer exists.'
    } else {
      error.value = e instanceof ApiError ? e.message : 'Failed to load cycle.'
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await load()
  try {
    library.value = await workoutsApi.list()
  } catch {
    // A failed library load only disables the add/swap pickers; the rest works.
  }
})

const filteredLibrary = computed(() => {
  const q = librarySearch.value.trim().toLowerCase()
  if (!q) return library.value
  return library.value.filter((w) => w.name.toLowerCase().includes(q))
})

async function addWorkout() {
  if (!cycle.value || pickWorkout.value === '') return
  try {
    cycle.value = await cyclesApi.addWorkout(cycle.value.id, {
      workout_id: pickWorkout.value,
      week: addForm.week || null,
      phase: addForm.phase.trim() || null,
      frequency: addForm.frequency.trim() || null,
      intensity: addForm.intensity.trim() || null,
      conditions: addForm.conditions.trim() || null,
    })
    pickWorkout.value = ''
    addForm.week = null
    addForm.phase = ''
    addForm.frequency = ''
    addForm.intensity = ''
    addForm.conditions = ''
    librarySearch.value = ''
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to add workout.'
  }
}

function startEdit(cw: CycleWorkout) {
  editingEntry.value = cw.id
  editForm.workout = cw.workout_id
  editForm.week = cw.week
  editForm.phase = cw.phase ?? ''
  editForm.frequency = cw.frequency ?? ''
  editForm.intensity = cw.intensity ?? ''
  editForm.conditions = cw.conditions ?? ''
}

async function saveEdit(entryId: number) {
  if (!cycle.value) return
  const patch: CycleWorkoutPatch = {
    week: editForm.week || null,
    phase: editForm.phase.trim() || null,
    frequency: editForm.frequency.trim() || null,
    intensity: editForm.intensity.trim() || null,
    conditions: editForm.conditions.trim() || null,
  }
  // A changed workout selection is the swap — prescription carries over server-side.
  if (editForm.workout !== '') patch.workout_id = editForm.workout
  try {
    cycle.value = await cyclesApi.updateWorkout(cycle.value.id, entryId, patch)
    editingEntry.value = null
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save entry.'
  }
}

async function move(index: number, delta: number) {
  if (!cycle.value) return
  const entries = cycle.value.workouts
  const target = index + delta
  if (target < 0 || target >= entries.length) return
  const order = entries.map((cw) => cw.id)
  ;[order[index], order[target]] = [order[target], order[index]]
  try {
    cycle.value = await cyclesApi.reorderWorkouts(cycle.value.id, order)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to reorder.'
  }
}

async function removeEntry(cw: CycleWorkout) {
  if (!cycle.value) return
  if (!window.confirm(`Remove “${cw.workout_name}” from this cycle?`)) return
  try {
    cycle.value = await cyclesApi.removeWorkout(cycle.value.id, cw.id)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to remove.'
  }
}

function onSaved(saved: Cycle) {
  // The edit modal returns cycle-level fields; preserve the loaded workout sequence.
  if (cycle.value) saved.workouts = cycle.value.workouts
  cycle.value = saved
  showEdit.value = false
}

async function removeCycle() {
  if (!cycle.value) return
  if (!window.confirm(`Delete “${cycle.value.name}”? This cannot be undone.`)) return
  try {
    await cyclesApi.remove(cycle.value.id)
    router.push({ name: 'cycles' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete.'
  }
}

function goToWorkout(workoutId: number) {
  router.push({ name: 'workout-detail', params: { id: workoutId } })
}
</script>

<template>
  <section class="detail">
    <RouterLink
      class="detail__back"
      :to="{ name: 'cycles' }">
      ← Cycles
    </RouterLink>

    <p
      v-if="loading"
      class="detail__status">
      Loading…
    </p>
    <p
      v-else-if="error && !cycle"
      class="detail__status detail__status--error">
      {{ error }}
    </p>

    <template v-else-if="cycle">
      <header class="detail__head">
        <div>
          <h1 class="detail__title">{{ cycle.name }}</h1>
          <span
            class="detail__status-badge"
            :class="`detail__status-badge--${cycle.status}`">
            {{ cycle.status }}
          </span>
        </div>
        <button
          type="button"
          class="btn"
          @click="showEdit = true">
          Edit
        </button>
      </header>

      <dl class="facts">
        <div
          v-if="cycle.goal_summary"
          class="facts__row">
          <dt>Goal</dt>
          <dd>{{ cycle.goal_summary }}</dd>
        </div>
        <div
          v-if="cycleTargetSummary(cycle)"
          class="facts__row">
          <dt>Target</dt>
          <dd>{{ cycleTargetSummary(cycle) }}</dd>
        </div>
        <div
          v-if="cycle.start_date"
          class="facts__row">
          <dt>Start</dt>
          <dd>{{ cycle.start_date }}</dd>
        </div>
        <div
          v-if="cycle.target_date"
          class="facts__row">
          <dt>Target date</dt>
          <dd>{{ cycle.target_date }}</dd>
        </div>
      </dl>

      <p
        v-if="error"
        class="detail__status detail__status--error">
        {{ error }}
      </p>

      <section
        v-if="cycle.notes"
        class="prose-block">
        <h2 class="section-title">Notes</h2>
        <!-- eslint-disable-next-line vue/no-v-html -- first-party markdown, html disabled in the renderer -->
        <div
          class="prose"
          v-html="renderMarkdown(cycle.notes)" />
      </section>

      <section class="composition">
        <h2 class="section-title">Workout sequence</h2>

        <p
          v-if="cycle.workouts.length === 0"
          class="composition__empty">
          No workouts yet. Add one below.
        </p>

        <ol
          v-else
          class="entries">
          <li
            v-for="(cw, i) in cycle.workouts"
            :key="cw.id"
            class="entry">
            <div class="entry__row">
              <span class="entry__pos">{{ i + 1 }}</span>
              <div class="entry__body">
                <button
                  type="button"
                  class="entry__name"
                  @click="goToWorkout(cw.workout_id)">
                  {{ cw.workout_name }}
                </button>
                <span
                  v-if="cyclePrescriptionSummary(cw)"
                  class="entry__rx">
                  {{ cyclePrescriptionSummary(cw) }}
                </span>
                <span
                  v-if="cw.conditions"
                  class="entry__conditions">
                  ⤷ {{ cw.conditions }}
                </span>
              </div>
              <div class="entry__reorder">
                <button
                  type="button"
                  class="icon-btn"
                  :disabled="i === 0"
                  aria-label="Move up"
                  @click="move(i, -1)">
                  ↑
                </button>
                <button
                  type="button"
                  class="icon-btn"
                  :disabled="i === cycle.workouts.length - 1"
                  aria-label="Move down"
                  @click="move(i, 1)">
                  ↓
                </button>
              </div>
            </div>

            <div class="entry__actions">
              <button
                type="button"
                class="link-btn"
                @click="startEdit(cw)">
                Edit / swap
              </button>
              <button
                type="button"
                class="link-btn link-btn--danger"
                @click="removeEntry(cw)">
                Remove
              </button>
            </div>

            <!-- Inline periodization editor, with the workout swap folded in. -->
            <div
              v-if="editingEntry === cw.id"
              class="panel">
              <label class="field">
                <span class="field__label">Workout (change to swap — prescription carries over)</span>
                <select
                  v-model="editForm.workout"
                  class="field__input">
                  <option
                    v-for="w in library"
                    :key="w.id"
                    :value="w.id">
                    {{ w.name }}
                  </option>
                </select>
              </label>
              <div class="rx-grid">
                <label class="field field--inline">
                  <span class="field__label">Week</span>
                  <input
                    v-model.number="editForm.week"
                    class="field__input"
                    type="number"
                    min="0" />
                </label>
                <label class="field field--inline">
                  <span class="field__label">Phase</span>
                  <input
                    v-model="editForm.phase"
                    class="field__input"
                    type="text"
                    placeholder="base, build, taper" />
                </label>
                <label class="field field--inline">
                  <span class="field__label">Frequency</span>
                  <input
                    v-model="editForm.frequency"
                    class="field__input"
                    type="text"
                    placeholder="3×/week" />
                </label>
                <label class="field field--inline">
                  <span class="field__label">Intensity</span>
                  <input
                    v-model="editForm.intensity"
                    class="field__input"
                    type="text"
                    placeholder="easy / Zone 2" />
                </label>
              </div>
              <label class="field">
                <span class="field__label">Conditions to advance</span>
                <input
                  v-model="editForm.conditions"
                  class="field__input"
                  type="text"
                  placeholder="when knee-to-wall symmetric, advance" />
              </label>
              <div class="panel__actions">
                <button
                  type="button"
                  class="btn"
                  @click="editingEntry = null">
                  Cancel
                </button>
                <button
                  type="button"
                  class="btn btn--accent"
                  @click="saveEdit(cw.id)">
                  Save
                </button>
              </div>
            </div>
          </li>
        </ol>

        <div class="add-workout">
          <h3 class="add-workout__title">Add a workout</h3>
          <input
            v-model="librarySearch"
            class="field__input"
            type="search"
            placeholder="Filter workouts…"
            aria-label="Filter workouts" />
          <select
            v-model="pickWorkout"
            class="field__input"
            aria-label="Workout to add">
            <option value="">Choose a workout…</option>
            <option
              v-for="w in filteredLibrary"
              :key="w.id"
              :value="w.id">
              {{ w.name }}
            </option>
          </select>
          <div class="rx-grid">
            <label class="field field--inline">
              <span class="field__label">Week</span>
              <input
                v-model.number="addForm.week"
                class="field__input"
                type="number"
                min="0" />
            </label>
            <label class="field field--inline">
              <span class="field__label">Phase</span>
              <input
                v-model="addForm.phase"
                class="field__input"
                type="text"
                placeholder="base" />
            </label>
            <label class="field field--inline">
              <span class="field__label">Frequency</span>
              <input
                v-model="addForm.frequency"
                class="field__input"
                type="text"
                placeholder="3×/week" />
            </label>
            <label class="field field--inline">
              <span class="field__label">Intensity</span>
              <input
                v-model="addForm.intensity"
                class="field__input"
                type="text"
                placeholder="easy / Zone 2" />
            </label>
          </div>
          <button
            type="button"
            class="btn btn--accent"
            :disabled="pickWorkout === ''"
            @click="addWorkout">
            + Add to cycle
          </button>
        </div>
      </section>

      <div class="detail__actions">
        <button
          type="button"
          class="btn btn--danger"
          @click="removeCycle">
          Delete cycle
        </button>
      </div>

      <AddEditCycleModal
        v-if="showEdit"
        :cycle="cycle"
        @saved="onSaved"
        @close="showEdit = false" />
    </template>
  </section>
</template>

<style scoped lang="scss">
.detail {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.detail__back {
  color: var(--text-muted);
  font-size: 0.9rem;
}

.detail__status {
  padding: var(--space-4);
  text-align: center;
  color: var(--text-muted);

  &--error {
    color: #f87171;
  }
}

.detail__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.detail__title {
  margin: 0;
  font-size: 1.6rem;
}

.detail__status-badge {
  display: inline-block;
  margin-top: var(--space-2);
  padding: 2px var(--space-2);
  border-radius: 999px;
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;

  &--active {
    background: var(--accent);
    color: var(--accent-contrast);
  }
}

.facts {
  margin: 0;
  display: grid;
  gap: var(--space-2);
}

.facts__row {
  display: flex;
  gap: var(--space-3);

  dt {
    flex: 0 0 6rem;
    color: var(--text-muted);
    font-size: 0.85rem;
  }

  dd {
    margin: 0;
  }
}

.section-title {
  margin: 0 0 var(--space-2);
  font-size: 1.05rem;
}

.composition {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.composition__empty {
  margin: 0;
  color: var(--text-muted);
}

.entries {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.entry {
  padding: var(--space-3);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.entry__row {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
}

.entry__pos {
  flex: 0 0 auto;
  min-width: 1.5rem;
  height: 1.5rem;
  display: grid;
  place-items: center;
  border-radius: 999px;
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
}

.entry__body {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.entry__name {
  align-self: flex-start;
  border: none;
  background: transparent;
  padding: 0;
  color: var(--text);
  font-weight: 600;
  font-size: 1.02rem;
  text-align: left;
  text-decoration: underline;
  text-decoration-color: var(--border);
}

.entry__rx {
  color: var(--text-muted);
  font-size: 0.85rem;
}

.entry__conditions {
  color: var(--accent);
  font-size: 0.78rem;
}

.entry__reorder {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.icon-btn {
  min-width: 2rem;
  min-height: 2rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  line-height: 1;

  &:disabled {
    opacity: 0.35;
  }
}

.entry__actions {
  display: flex;
  gap: var(--space-3);
  margin-top: var(--space-2);
  padding-top: var(--space-2);
  border-top: 1px dashed var(--border);
}

.link-btn {
  border: none;
  background: transparent;
  color: var(--accent);
  padding: 0;
  font-size: 0.85rem;

  &--danger {
    color: #f87171;
  }
}

.panel {
  margin-top: var(--space-2);
  padding: var(--space-2) 0 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.panel__actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-2);
}

.rx-grid {
  display: grid;
  gap: var(--space-2);
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.field__label {
  font-size: 0.8rem;
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

.field--inline {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);

  .field__input {
    max-width: 10rem;
  }
}

.add-workout {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  background: var(--surface);
  border: 1px dashed var(--border);
  border-radius: var(--radius);
}

.add-workout__title {
  margin: 0;
  font-size: 0.95rem;
}

.detail__actions {
  display: flex;
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

  &--danger {
    color: #f87171;
    border-color: #f87171;
    background: transparent;
  }

  &:disabled {
    opacity: 0.5;
  }
}

@media (min-width: 720px) {
  .rx-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
