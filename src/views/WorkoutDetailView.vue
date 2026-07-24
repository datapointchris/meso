<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { workoutsApi, prescriptionSummary, type Workout, type WorkoutMovement, type WorkoutMovementPatch } from '@/api/workouts'
import { movementsApi, type Movement, type RelatedMovement } from '@/api/movements'
import { sessionsApi } from '@/api/sessions'
import { ApiError } from '@/api/client'
import { renderMarkdown } from '@/composables/useMarkdown'
import AddEditWorkoutModal from '@/components/AddEditWorkoutModal.vue'

const route = useRoute()
const router = useRouter()

const workout = ref<Workout | null>(null)
const loading = ref(true)
const error = ref('')
const showEdit = ref(false)

const id = computed(() => Number(route.params.id))

// The movement library, loaded once to populate the "add movement" picker.
const library = ref<Movement[]>([])
const pickMovement = ref<number | ''>('')
const addForm = reactive({ sets: null as number | null, reps: '', load: '' })
const librarySearch = ref('')

// Inline prescription editing: which entry is open, and its edit buffer.
const editingEntry = ref<number | null>(null)
const editForm = reactive({ sets: null as number | null, reps: '', load: '', rest: null as number | null, superset: '', notes: '' })

// Swap: which entry is choosing a substitute, and the candidate movements (that
// movement's relationships).
const swapFor = ref<number | null>(null)
const swapCandidates = ref<RelatedMovement[]>([])
const swapLoading = ref(false)
const swapError = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    workout.value = await workoutsApi.get(id.value)
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      error.value = 'That workout no longer exists.'
    } else {
      error.value = e instanceof ApiError ? e.message : 'Failed to load workout.'
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await load()
  try {
    library.value = await movementsApi.list()
  } catch {
    // A failed library load only disables the add picker; the rest works.
  }
})

const filteredLibrary = computed(() => {
  const q = librarySearch.value.trim().toLowerCase()
  if (!q) return library.value
  return library.value.filter((m) => m.name.toLowerCase().includes(q))
})

async function addMovement() {
  if (!workout.value || pickMovement.value === '') return
  try {
    workout.value = await workoutsApi.addMovement(workout.value.id, {
      movement_id: pickMovement.value,
      sets: addForm.sets || null,
      reps: addForm.reps.trim() || null,
      load: addForm.load.trim() || null,
    })
    pickMovement.value = ''
    addForm.sets = null
    addForm.reps = ''
    addForm.load = ''
    librarySearch.value = ''
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to add movement.'
  }
}

function startEdit(m: WorkoutMovement) {
  swapFor.value = null
  editingEntry.value = m.id
  editForm.sets = m.sets
  editForm.reps = m.reps ?? ''
  editForm.load = m.load ?? ''
  editForm.rest = m.rest_seconds
  editForm.superset = m.superset_group ?? ''
  editForm.notes = m.notes
}

async function saveEdit(entryId: number) {
  if (!workout.value) return
  const patch: WorkoutMovementPatch = {
    sets: editForm.sets || null,
    reps: editForm.reps.trim() || null,
    load: editForm.load.trim() || null,
    rest_seconds: editForm.rest || null,
    superset_group: editForm.superset.trim() || null,
    notes: editForm.notes,
  }
  try {
    workout.value = await workoutsApi.updateMovement(workout.value.id, entryId, patch)
    editingEntry.value = null
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save entry.'
  }
}

// startSwap loads the entry's movement to read its relationships — the candidate
// substitutes. Any related movement is offerable; alternates are the common case.
async function startSwap(m: WorkoutMovement) {
  editingEntry.value = null
  swapFor.value = m.id
  swapCandidates.value = []
  swapError.value = ''
  swapLoading.value = true
  try {
    const full = await movementsApi.get(m.movement_id)
    swapCandidates.value = full.related
  } catch (e) {
    swapError.value = e instanceof ApiError ? e.message : 'Failed to load alternates.'
  } finally {
    swapLoading.value = false
  }
}

async function applySwap(entryId: number, candidate: RelatedMovement) {
  if (!workout.value) return
  try {
    // Only movement_id changes — the prescription (sets/reps/load) carries over.
    workout.value = await workoutsApi.updateMovement(workout.value.id, entryId, { movement_id: candidate.id })
    swapFor.value = null
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to swap.'
  }
}

async function move(index: number, delta: number) {
  if (!workout.value) return
  const entries = workout.value.movements
  const target = index + delta
  if (target < 0 || target >= entries.length) return
  const order = entries.map((m) => m.id)
  ;[order[index], order[target]] = [order[target], order[index]]
  try {
    workout.value = await workoutsApi.reorderMovements(workout.value.id, order)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to reorder.'
  }
}

async function removeEntry(m: WorkoutMovement) {
  if (!workout.value) return
  if (!window.confirm(`Remove “${m.movement_name}” from this workout?`)) return
  try {
    workout.value = await workoutsApi.removeMovement(workout.value.id, m.id)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to remove.'
  }
}

function onSaved(saved: Workout) {
  // The edit modal returns workout-level fields; preserve the loaded movement list.
  if (workout.value) saved.movements = workout.value.movements
  workout.value = saved
  showEdit.value = false
}

async function removeWorkout() {
  if (!workout.value) return
  if (!window.confirm(`Delete “${workout.value.name}”? This cannot be undone.`)) return
  try {
    await workoutsApi.remove(workout.value.id)
    router.push({ name: 'workouts' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete.'
  }
}

function goToMovement(movementId: number) {
  router.push({ name: 'movement-detail', params: { id: movementId } })
}

// startSession logs a session from this workout — its movements copy in with the
// prescription seeded as actuals — and jumps to the mobile logging screen.
const starting = ref(false)
async function startSession() {
  if (!workout.value || starting.value) return
  starting.value = true
  try {
    const session = await sessionsApi.create({ workout_id: workout.value.id })
    router.push({ name: 'session-detail', params: { id: session.id } })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to start session.'
    starting.value = false
  }
}
</script>

<template>
  <section class="detail">
    <RouterLink
      class="detail__back"
      :to="{ name: 'workouts' }">
      ← Workouts
    </RouterLink>

    <p
      v-if="loading"
      class="detail__status">
      Loading…
    </p>
    <p
      v-else-if="error && !workout"
      class="detail__status detail__status--error">
      {{ error }}
    </p>

    <template v-else-if="workout">
      <header class="detail__head">
        <div>
          <h1 class="detail__title">{{ workout.name }}</h1>
          <span
            v-if="workout.theme"
            class="detail__theme">
            {{ workout.theme }}
          </span>
        </div>
        <button
          type="button"
          class="btn"
          @click="showEdit = true">
          Edit
        </button>
      </header>

      <button
        type="button"
        class="btn btn--accent detail__start"
        :disabled="starting"
        @click="startSession">
        {{ starting ? 'Starting…' : '▶ Start session' }}
      </button>

      <div
        v-if="workout.tags.length"
        class="chips-row">
        <span
          v-for="tag in workout.tags"
          :key="tag"
          class="tag">
          {{ tag }}
        </span>
      </div>

      <p
        v-if="error"
        class="detail__status detail__status--error">
        {{ error }}
      </p>

      <section
        v-if="workout.notes"
        class="prose-block">
        <h2 class="section-title">Notes</h2>
        <!-- eslint-disable-next-line vue/no-v-html -- first-party markdown, html disabled in the renderer -->
        <div
          class="prose"
          v-html="renderMarkdown(workout.notes)" />
      </section>

      <section class="composition">
        <h2 class="section-title">Movements</h2>

        <p
          v-if="workout.movements.length === 0"
          class="composition__empty">
          No movements yet. Add one below.
        </p>

        <ol
          v-else
          class="entries">
          <li
            v-for="(m, i) in workout.movements"
            :key="m.id"
            class="entry">
            <div class="entry__row">
              <span class="entry__pos">{{ i + 1 }}</span>
              <div class="entry__body">
                <button
                  type="button"
                  class="entry__name"
                  @click="goToMovement(m.movement_id)">
                  {{ m.movement_name }}
                </button>
                <span
                  v-if="prescriptionSummary(m)"
                  class="entry__rx">
                  {{ prescriptionSummary(m) }}
                </span>
                <span
                  v-if="m.superset_group"
                  class="entry__superset">
                  superset {{ m.superset_group }}
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
                  :disabled="i === workout.movements.length - 1"
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
                @click="startSwap(m)">
                Swap
              </button>
              <button
                type="button"
                class="link-btn"
                @click="startEdit(m)">
                Edit
              </button>
              <button
                type="button"
                class="link-btn link-btn--danger"
                @click="removeEntry(m)">
                Remove
              </button>
            </div>

            <!-- Swap picker: the entry's movement's relationships. -->
            <div
              v-if="swapFor === m.id"
              class="panel">
              <p
                v-if="swapLoading"
                class="panel__status">
                Loading alternates…
              </p>
              <p
                v-else-if="swapError"
                class="panel__status panel__status--error">
                {{ swapError }}
              </p>
              <template v-else>
                <p
                  v-if="swapCandidates.length === 0"
                  class="panel__status">
                  No alternates defined for {{ m.movement_name }}. Add relationships on its
                  <button
                    type="button"
                    class="link-btn"
                    @click="goToMovement(m.movement_id)">
                    movement page
                  </button>
                  .
                </p>
                <ul
                  v-else
                  class="swap-list">
                  <li
                    v-for="c in swapCandidates"
                    :key="`${c.id}-${c.relationship_kind}`">
                    <button
                      type="button"
                      class="swap-option"
                      @click="applySwap(m.id, c)">
                      <span class="swap-option__name">{{ c.name }}</span>
                      <span class="swap-option__kind">{{ c.relationship_kind }}</span>
                    </button>
                  </li>
                </ul>
                <p class="panel__hint">The prescription carries over to the swap.</p>
              </template>
            </div>

            <!-- Inline prescription editor. -->
            <div
              v-if="editingEntry === m.id"
              class="panel">
              <div class="rx-grid">
                <label class="field field--inline">
                  <span class="field__label">Sets</span>
                  <input
                    v-model.number="editForm.sets"
                    class="field__input"
                    type="number"
                    min="0" />
                </label>
                <label class="field field--inline">
                  <span class="field__label">Reps</span>
                  <input
                    v-model="editForm.reps"
                    class="field__input"
                    type="text"
                    placeholder="4–6, AMRAP, 30s" />
                </label>
                <label class="field field--inline">
                  <span class="field__label">Load</span>
                  <input
                    v-model="editForm.load"
                    class="field__input"
                    type="text"
                    placeholder="80% 1RM" />
                </label>
                <label class="field field--inline">
                  <span class="field__label">Rest (s)</span>
                  <input
                    v-model.number="editForm.rest"
                    class="field__input"
                    type="number"
                    min="0" />
                </label>
                <label class="field field--inline">
                  <span class="field__label">Superset</span>
                  <input
                    v-model="editForm.superset"
                    class="field__input"
                    type="text" />
                </label>
              </div>
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
                  @click="saveEdit(m.id)">
                  Save
                </button>
              </div>
            </div>
          </li>
        </ol>

        <div class="add-movement">
          <h3 class="add-movement__title">Add a movement</h3>
          <input
            v-model="librarySearch"
            class="field__input"
            type="search"
            placeholder="Filter the library…"
            aria-label="Filter movements" />
          <select
            v-model="pickMovement"
            class="field__input"
            aria-label="Movement to add">
            <option value="">Choose a movement…</option>
            <option
              v-for="m in filteredLibrary"
              :key="m.id"
              :value="m.id">
              {{ m.name }}
            </option>
          </select>
          <div class="rx-grid">
            <label class="field field--inline">
              <span class="field__label">Sets</span>
              <input
                v-model.number="addForm.sets"
                class="field__input"
                type="number"
                min="0" />
            </label>
            <label class="field field--inline">
              <span class="field__label">Reps</span>
              <input
                v-model="addForm.reps"
                class="field__input"
                type="text"
                placeholder="5, AMRAP" />
            </label>
            <label class="field field--inline">
              <span class="field__label">Load</span>
              <input
                v-model="addForm.load"
                class="field__input"
                type="text"
                placeholder="80% 1RM" />
            </label>
          </div>
          <button
            type="button"
            class="btn btn--accent"
            :disabled="pickMovement === ''"
            @click="addMovement">
            + Add to workout
          </button>
        </div>
      </section>

      <div class="detail__actions">
        <button
          type="button"
          class="btn btn--danger"
          @click="removeWorkout">
          Delete workout
        </button>
      </div>

      <AddEditWorkoutModal
        v-if="showEdit"
        :workout="workout"
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

.detail__theme {
  display: inline-block;
  margin-top: var(--space-2);
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

// The primary CTA — full-width so it's a one-tap target on a phone.
.detail__start {
  width: 100%;
  justify-content: center;
}

.chips-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.tag {
  padding: 2px var(--space-2);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 0.75rem;
}

.section-title {
  margin: 0 0 var(--space-2);
  font-size: 1.05rem;
}

.prose {
  color: var(--text);

  :deep(p) {
    margin: 0 0 var(--space-2);
  }

  :deep(ol),
  :deep(ul) {
    margin: 0 0 var(--space-2);
    padding-left: var(--space-6);
  }
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
  counter-reset: none;
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

.entry__superset {
  color: var(--accent);
  font-size: 0.75rem;
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
}

.panel__status {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.85rem;

  &--error {
    color: #f87171;
  }
}

.panel__hint {
  margin: var(--space-1) 0 0;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.panel__actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-2);
}

.swap-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.swap-option {
  width: 100%;
  min-height: var(--touch-target);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
}

.swap-option__kind {
  color: var(--text-muted);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
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

.add-movement {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  background: var(--surface);
  border: 1px dashed var(--border);
  border-radius: var(--radius);
}

.add-movement__title {
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
