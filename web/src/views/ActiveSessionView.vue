<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  sessionsApi,
  setCount,
  isInProgress,
  targetSummary,
  previousSummary,
  type Session,
  type SessionMovement,
  type SessionMovementPatch,
  type SessionSet,
  type SessionSetInput,
} from '@/api/sessions'
import type { Movement } from '@/api/movements'
import type { Workout } from '@/api/workouts'
import { ApiError } from '@/api/client'
import { useConfirm } from '@/composables/useConfirm'
import MovementPicker from '@/components/MovementPicker.vue'
import PromoteSessionModal from '@/components/PromoteSessionModal.vue'

// The mobile-critical logging screen. One movement, one card, one big "Log set" button:
// the common case is another set exactly like the last, and it should cost a tap.
//
// The card shows the plan as a line of text rather than as inputs. A session is a record
// of what happened, and the plan is context for it — editable, but not the thing being
// filled in.
const route = useRoute()
const router = useRouter()
const { ask } = useConfirm()
const id = computed(() => String(route.params.id))

const session = ref<Session | null>(null)
const loading = ref(true)
const error = ref('')
const busyEntry = ref<number | null>(null)

// Session-level edit buffer, synced from the loaded session and saved on blur.
const meta = reactive({ felt: '', duration: null as number | null, notes: '' })

async function load() {
  loading.value = true
  error.value = ''
  try {
    apply(await sessionsApi.get(id.value))
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      error.value = 'That session no longer exists.'
    } else {
      error.value = e instanceof ApiError ? e.message : 'Failed to load session.'
    }
  } finally {
    loading.value = false
  }
}

// apply swaps in a server response. The meta buffer is only re-synced on the first load:
// re-syncing on every write would overwrite what is being typed right now, since a set
// logged mid-sentence returns the whole session.
function apply(s: Session, syncMeta = false) {
  const first = session.value === null
  session.value = s
  if (first || syncMeta) {
    meta.felt = s.felt ?? ''
    meta.duration = s.duration_minutes
    meta.notes = s.overall_notes
  }
}

onMounted(load)

// What happened, not a score against the plan.
const summary = computed(() => {
  if (!session.value) return ''
  const movements = session.value.movements.length
  const sets = setCount(session.value)
  return `${movements} movement${movements === 1 ? '' : 's'} · ${sets} set${sets === 1 ? '' : 's'}`
})

const live = computed(() => session.value !== null && isInProgress(session.value))

// Only a session with no template can be promoted — one already backed by a workout
// would silently fork it.
const isFreeForm = computed(() => session.value !== null && session.value.workout_id === null)

const showPicker = ref(false)
const showPromote = ref(false)
const finishing = ref(false)
const editingSet = ref<{ entryId: number; set: SessionSet } | null>(null)

async function withEntry<T>(entryId: number, work: () => Promise<T>) {
  if (busyEntry.value !== null) return
  busyEntry.value = entryId
  try {
    return await work()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save.'
  } finally {
    busyEntry.value = null
  }
}

// logSet is the screen's whole point. An empty body tells the server to repeat the
// previous set, so nothing has to be typed unless something actually changed.
function logSet(entry: SessionMovement, body: SessionSetInput = {}) {
  return withEntry(entry.id, async () => {
    apply(await sessionsApi.addSet(id.value, entry.id, body))
  })
}

function patchEntry(entry: SessionMovement, patch: SessionMovementPatch) {
  return withEntry(entry.id, async () => {
    apply(await sessionsApi.updateMovement(id.value, entry.id, patch))
  })
}

// The checkbox ticks itself when the logged sets reach the plan. It stays tappable so
// stopping short on purpose can be said, and so a movement done without logging sets can
// still be marked.
function toggleDone(entry: SessionMovement) {
  return patchEntry(entry, { done: !entry.done })
}

async function saveSet(patch: SessionSetInput) {
  const editing = editingSet.value
  if (!editing) return
  await withEntry(editing.entryId, async () => {
    apply(await sessionsApi.updateSet(id.value, editing.entryId, editing.set.id, patch))
  })
  editingSet.value = null
}

async function removeSet(entryId: number, set: SessionSet) {
  const ok = await ask({
    title: 'Remove set',
    message: `Remove set ${set.position}?`,
    confirmLabel: 'Remove',
    danger: true,
  })
  if (!ok) return
  await withEntry(entryId, async () => {
    apply(await sessionsApi.removeSet(id.value, entryId, set.id))
  })
  editingSet.value = null
}

// Adding and removing movements works on a session from a template too. Doing an extra
// movement, or skipping one, is part of what happened — the record has to be able to
// say so, and editing the workout instead would rewrite every past session's plan.
async function addMovement(movement: Movement) {
  if (busyEntry.value !== null) return
  busyEntry.value = -1
  try {
    apply(await sessionsApi.addMovement(id.value, { movement_id: movement.id }))
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to add movement.'
  } finally {
    busyEntry.value = null
  }
}

async function removeEntry(entry: SessionMovement) {
  const ok = await ask({
    title: 'Remove movement',
    message: `Remove “${entry.movement_name}” from this session? Its ${entry.sets.length} logged set${
      entry.sets.length === 1 ? '' : 's'
    } go too.`,
    confirmLabel: 'Remove',
    danger: true,
  })
  if (!ok) return
  await withEntry(entry.id, async () => {
    apply(await sessionsApi.removeMovement(id.value, entry.id))
  })
}

async function finish() {
  if (finishing.value) return
  finishing.value = true
  try {
    await sessionsApi.finish(id.value)
    router.push({ name: 'sessions' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to finish the session.'
    finishing.value = false
  }
}

async function deleteSession() {
  const ok = await ask({
    title: 'Delete session',
    message: 'Delete this session? Everything logged in it goes too.',
    confirmLabel: 'Delete',
    danger: true,
  })
  if (!ok) return
  try {
    await sessionsApi.remove(id.value)
    router.push({ name: 'sessions' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete the session.'
  }
}

function onPromoted(workout: Workout) {
  showPromote.value = false
  router.push({ name: 'workout-detail', params: { id: workout.id } })
}

// saveMeta persists the session-level fields (felt / duration / notes) on blur.
async function saveMeta() {
  if (!session.value) return
  try {
    apply(
      await sessionsApi.update(id.value, {
        felt: meta.felt.trim() || null,
        duration_minutes: meta.duration || null,
        overall_notes: meta.notes,
      }),
    )
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save session.'
  }
}

// Load mode decides what a set is even described by, so the inputs follow it rather
// than showing everything and letting the irrelevant ones sit there empty.
function asksForLoad(entry: SessionMovement): boolean {
  return entry.load_mode === 'weighted' || entry.load_mode === 'assisted'
}
function asksForHold(entry: SessionMovement): boolean {
  return entry.load_mode === 'timed'
}
function loadLabel(entry: SessionMovement): string {
  return entry.load_mode === 'assisted' ? 'Assistance' : 'Load'
}

function setSummary(set: SessionSet): string {
  const parts: string[] = []
  if (set.reps != null) parts.push(`${set.reps} reps`)
  if (set.hold_seconds != null) parts.push(`${set.hold_seconds}s`)
  if (set.load) parts.push(set.load)
  return parts.length > 0 ? parts.join(' · ') : 'logged'
}

// The edit sheet's own buffer, so typing in it never fights a server response.
const setEdit = reactive({ reps: '', load: '', hold: '' })
function openSetEditor(entryId: number, set: SessionSet) {
  editingSet.value = { entryId, set }
  setEdit.reps = set.reps != null ? String(set.reps) : ''
  setEdit.load = set.load ?? ''
  setEdit.hold = set.hold_seconds != null ? String(set.hold_seconds) : ''
}
function submitSetEdit() {
  saveSet({
    reps: setEdit.reps.trim() === '' ? null : Number(setEdit.reps),
    load: setEdit.load.trim() || null,
    hold_seconds: setEdit.hold.trim() === '' ? null : Number(setEdit.hold),
  })
}
</script>

<template>
  <section class="session">
    <RouterLink
      class="session__back"
      :to="{ name: 'sessions' }">
      ← Sessions
    </RouterLink>

    <p
      v-if="loading"
      class="session__status">
      Loading…
    </p>
    <p
      v-else-if="error && !session"
      class="session__status session__status--error">
      {{ error }}
    </p>

    <template v-else-if="session">
      <header class="session__head">
        <div>
          <h1 class="session__title">{{ session.workout_name ?? 'Free-form session' }}</h1>
          <span class="session__date">{{ session.performed_on }}</span>
        </div>
        <span class="session__summary">{{ summary }}</span>
      </header>

      <p
        v-if="error"
        class="session__status session__status--error">
        {{ error }}
      </p>

      <p
        v-if="session.movements.length === 0"
        class="session__status">
        Nothing logged yet. Add the first movement below.
      </p>

      <ol
        v-else
        class="log">
        <li
          v-for="entry in session.movements"
          :key="entry.id"
          class="entry"
          :class="{ 'entry--done': entry.done }">
          <div class="entry__top">
            <button
              type="button"
              class="check"
              :class="{ 'check--on': entry.done }"
              :aria-pressed="entry.done"
              :aria-label="entry.done ? `Mark ${entry.movement_name} not done` : `Mark ${entry.movement_name} done`"
              @click="toggleDone(entry)">
              {{ entry.done ? '✓' : '' }}
            </button>
            <div class="entry__ident">
              <span class="entry__name">{{ entry.movement_name }}</span>
              <span class="entry__context">
                <span v-if="targetSummary(entry)">Plan: {{ targetSummary(entry) }}</span>
                <span
                  v-if="entry.previous"
                  class="entry__previous">
                  Last: {{ previousSummary(entry) }}
                </span>
              </span>
            </div>
            <button
              type="button"
              class="entry__remove"
              :aria-label="`Remove ${entry.movement_name}`"
              @click="removeEntry(entry)">
              ✕
            </button>
          </div>

          <ul
            v-if="entry.sets.length > 0"
            class="sets">
            <li
              v-for="set in entry.sets"
              :key="set.id">
              <button
                type="button"
                class="set"
                :aria-label="`Edit set ${set.position}`"
                @click="openSetEditor(entry.id, set)">
                <span class="set__index">{{ set.position }}</span>
                <span class="set__values">{{ setSummary(set) }}</span>
                <span
                  v-if="set.set_kind !== 'working'"
                  class="set__kind">
                  {{ set.set_kind }}
                </span>
              </button>
            </li>
          </ul>

          <button
            type="button"
            class="log-set"
            :disabled="busyEntry === entry.id"
            @click="logSet(entry)">
            {{ busyEntry === entry.id ? 'Logging…' : '+ Log set' }}
          </button>

          <p class="entry__hint">
            {{ entry.sets.length === 0 ? 'Logs the plan, then repeats the last set.' : 'Repeats the last set. Tap a set to change it.' }}
          </p>
        </li>
      </ol>

      <section class="compose">
        <button
          v-if="!showPicker"
          type="button"
          class="btn compose__open"
          @click="showPicker = true">
          + Add a movement
        </button>
        <template v-else>
          <div class="compose__head">
            <h2 class="section-title">Add a movement</h2>
            <button
              type="button"
              class="link-btn"
              @click="showPicker = false">
              Close
            </button>
          </div>
          <MovementPicker
            :busy="busyEntry !== null"
            @pick="addMovement" />
        </template>
      </section>

      <section class="meta">
        <h2 class="section-title">How it went</h2>
        <div class="meta__grid">
          <label class="field">
            <span class="field__label">Felt</span>
            <input
              v-model="meta.felt"
              class="field__input"
              type="text"
              placeholder="strong, tired, loose"
              @blur="saveMeta" />
          </label>
          <label class="field">
            <span class="field__label">Duration (min)</span>
            <input
              v-model.number="meta.duration"
              class="field__input"
              type="number"
              min="0"
              inputmode="numeric"
              placeholder="filled in when you finish"
              @blur="saveMeta" />
          </label>
        </div>
        <label class="field">
          <span class="field__label">Session notes</span>
          <textarea
            v-model="meta.notes"
            class="field__input field__input--area"
            rows="3"
            placeholder="Overall notes for the session…"
            @blur="saveMeta" />
        </label>
      </section>

      <div class="session__secondary">
        <button
          v-if="isFreeForm && session.movements.length > 0"
          type="button"
          class="btn"
          @click="showPromote = true">
          Save as workout
        </button>
        <button
          type="button"
          class="btn btn--quiet"
          @click="deleteSession">
          Delete session
        </button>
      </div>

      <!-- Sticky so the one action that ends the session sits in the thumb zone rather
           than at the bottom of a page that grows with every set. -->
      <div
        v-if="live"
        class="finish-bar">
        <button
          type="button"
          class="btn btn--accent finish-bar__btn"
          :disabled="finishing"
          @click="finish">
          {{ finishing ? 'Finishing…' : 'Finish session' }}
        </button>
      </div>

      <PromoteSessionModal
        v-if="showPromote"
        :session-id="session.id"
        :movement-count="session.movements.length"
        @promoted="onPromoted"
        @close="showPromote = false" />

      <div
        v-if="editingSet"
        class="overlay"
        @click.self="editingSet = null">
        <form
          class="sheet"
          @submit.prevent="submitSetEdit">
          <h2 class="sheet__title">Set {{ editingSet.set.position }}</h2>
          <div class="sheet__grid">
            <label
              v-if="!asksForHold(session.movements.find((m) => m.id === editingSet!.entryId)!)"
              class="field">
              <span class="field__label">Reps</span>
              <input
                v-model="setEdit.reps"
                class="field__input"
                type="number"
                min="0"
                inputmode="numeric" />
            </label>
            <label
              v-if="asksForHold(session.movements.find((m) => m.id === editingSet!.entryId)!)"
              class="field">
              <span class="field__label">Hold (sec)</span>
              <input
                v-model="setEdit.hold"
                class="field__input"
                type="number"
                min="0"
                inputmode="numeric" />
            </label>
            <label
              v-if="asksForLoad(session.movements.find((m) => m.id === editingSet!.entryId)!)"
              class="field">
              <span class="field__label">{{ loadLabel(session.movements.find((m) => m.id === editingSet!.entryId)!) }}</span>
              <input
                v-model="setEdit.load"
                class="field__input"
                type="text"
                placeholder="100lb, 2 plates" />
            </label>
          </div>
          <div class="sheet__actions">
            <button
              type="button"
              class="btn btn--quiet"
              @click="removeSet(editingSet.entryId, editingSet.set)">
              Remove
            </button>
            <button
              type="button"
              class="btn"
              @click="editingSet = null">
              Cancel
            </button>
            <button
              type="submit"
              class="btn btn--accent">
              Save
            </button>
          </div>
        </form>
      </div>
    </template>
  </section>
</template>

<style scoped lang="scss">
.session {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  // Room for the sticky finish bar: its own offset off the tab bar, plus its height,
  // plus a gap. Without this the last card sits underneath it and cannot be tapped.
  padding-bottom: calc(var(--touch-target) * 2.3 + var(--space-6));
}

.session__back {
  color: var(--text-muted);
  font-size: 0.9rem;
}

.session__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-2);
}

.session__title {
  margin: 0;
  font-size: 1.35rem;
}

.session__date {
  color: var(--text-muted);
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
}

.session__summary {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
}

.session__status {
  padding: var(--space-4);
  color: var(--text-muted);
  text-align: center;

  &--error {
    color: var(--negative);
  }
}

.log {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.entry {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);

  &--done {
    border-color: var(--accent);
  }
}

.entry__top {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
}

.check {
  flex-shrink: 0;
  width: var(--touch-target);
  height: var(--touch-target);
  border: 2px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--accent-contrast);
  font-size: 1.2rem;
  line-height: 1;

  &--on {
    background: var(--accent);
    border-color: var(--accent);
  }
}

.entry__ident {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.entry__name {
  font-weight: 600;
  font-size: 1.05rem;
}

.entry__context {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  color: var(--text-muted);
  font-size: 0.78rem;
}

.entry__previous {
  color: var(--accent);
}

.entry__remove {
  flex-shrink: 0;
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.85rem;
}

.sets {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.set {
  width: 100%;
  min-height: var(--touch-target);
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  text-align: left;
}

.set__index {
  flex-shrink: 0;
  width: 1.25rem;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.set__values {
  flex: 1;
  min-width: 0;
  font-variant-numeric: tabular-nums;
}

.set__kind {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

// The largest target on the card: it is pressed once per set, mid-workout, one-handed.
.log-set {
  min-height: calc(var(--touch-target) * 1.15);
  border: 1px solid var(--accent);
  border-radius: var(--radius);
  background: var(--accent);
  color: var(--accent-contrast);
  font-weight: 700;
  font-size: 1.05rem;

  &:disabled {
    opacity: 0.6;
  }
}

.entry__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
  text-align: center;
}

.compose,
.meta {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.compose__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.compose__open {
  justify-content: center;
}

.section-title {
  margin: 0;
  font-size: 1rem;
}

.link-btn {
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--accent);
  font: inherit;
}

.meta__grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
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

  &--area {
    min-height: auto;
    resize: vertical;
  }
}

.session__secondary {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.finish-bar {
  position: fixed;
  left: 0;
  right: 0;
  // Clears the tab bar, which is itself fixed to the bottom edge.
  bottom: calc(var(--touch-target) + var(--space-4) + var(--safe-bottom));
  z-index: 20;
  display: flex;
  justify-content: center;
  // The right inset clears the feedback button, which is fixed at the same height.
  padding: 0 calc(var(--touch-target) + var(--space-3)) 0 var(--space-4);
  pointer-events: none;
}

.finish-bar__btn {
  pointer-events: auto;
  width: 100%;
  max-width: var(--content-max);
  min-height: calc(var(--touch-target) * 1.15);
  justify-content: center;
  font-size: 1.05rem;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.35);
}

.overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
}

.sheet {
  width: 100%;
  max-width: 32rem;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
  padding-bottom: calc(var(--space-4) + var(--safe-bottom));
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius) var(--radius) 0 0;
}

.sheet__title {
  margin: 0;
  font-size: 1.1rem;
}

.sheet__grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-2);
}

.sheet__actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

.btn {
  min-height: var(--touch-target);
  display: inline-flex;
  align-items: center;
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

  &--quiet {
    border-color: var(--negative);
    color: var(--negative);
    background: transparent;
  }

  &:disabled {
    opacity: 0.6;
  }
}

@media (min-width: 720px) {
  .overlay {
    align-items: center;
  }

  .sheet {
    border-radius: var(--radius);
  }

  .finish-bar {
    bottom: var(--space-4);
  }
}
</style>
