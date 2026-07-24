<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useRoute } from 'vue-router'
import { sessionsApi, doneCount, type Session, type SessionMovement, type SessionMovementPatch } from '@/api/sessions'
import { ApiError } from '@/api/client'

// The mobile-critical logging screen: check off sets one-handed, bump actual
// sets/reps/load against the plan the session was seeded with, and jot notes. Every
// edit PATCHes immediately and the server returns the refreshed session, so state is
// never lost on an app-switch (v1 is online; offline logging is deferred).
const route = useRoute()
const id = computed(() => String(route.params.id))

const session = ref<Session | null>(null)
const loading = ref(true)
const error = ref('')

// Session-level edit buffer, synced from the loaded session and saved on blur.
const meta = reactive({ felt: '', duration: null as number | null, notes: '' })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const s = await sessionsApi.get(id.value)
    session.value = s
    syncMeta(s)
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

function syncMeta(s: Session) {
  meta.felt = s.felt ?? ''
  meta.duration = s.duration_minutes
  meta.notes = s.overall_notes
}

onMounted(load)

const progress = computed(() => (session.value ? `${doneCount(session.value)} / ${session.value.movements.length}` : ''))

// patchEntry applies one field change to a logged movement and swaps in the refreshed
// session the server returns. Errors surface without discarding the loaded session.
async function patchEntry(entry: SessionMovement, patch: SessionMovementPatch) {
  if (!session.value) return
  try {
    session.value = await sessionsApi.updateMovement(session.value.id, entry.id, patch)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save.'
  }
}

function toggleDone(entry: SessionMovement) {
  patchEntry(entry, { done: !entry.done })
}

function bumpSets(entry: SessionMovement, delta: number) {
  const next = Math.max(0, (entry.actual_sets ?? 0) + delta)
  patchEntry(entry, { actual_sets: next })
}

// saveReps/saveLoad/saveEntryNotes fire on blur, sending the trimmed value (empty → null).
function saveReps(entry: SessionMovement, value: string) {
  const v = value.trim()
  if (v === (entry.actual_reps ?? '')) return
  patchEntry(entry, { actual_reps: v || null })
}
function saveLoad(entry: SessionMovement, value: string) {
  const v = value.trim()
  if (v === (entry.actual_load ?? '')) return
  patchEntry(entry, { actual_load: v || null })
}
function saveEntryNotes(entry: SessionMovement, value: string) {
  if (value === entry.notes) return
  patchEntry(entry, { notes: value })
}

// saveMeta persists the session-level fields (felt / duration / notes) on blur.
async function saveMeta() {
  if (!session.value) return
  try {
    session.value = await sessionsApi.update(session.value.id, {
      felt: meta.felt.trim() || null,
      duration_minutes: meta.duration || null,
      overall_notes: meta.notes,
    })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save session.'
  }
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
          <h1 class="session__title">{{ session.workout_name ?? 'Ad-hoc session' }}</h1>
          <span class="session__date">{{ session.performed_on }}</span>
        </div>
        <span
          class="session__progress"
          aria-label="Movements completed">
          {{ progress }}
        </span>
      </header>

      <p
        v-if="error"
        class="session__status session__status--error">
        {{ error }}
      </p>

      <p
        v-if="session.movements.length === 0"
        class="session__status">
        No movements were logged for this session.
      </p>

      <ol
        v-else
        class="log">
        <li
          v-for="entry in session.movements"
          :key="entry.id"
          class="log-entry"
          :class="{ 'log-entry--done': entry.done }">
          <div class="log-entry__top">
            <button
              type="button"
              class="check"
              :class="{ 'check--on': entry.done }"
              :aria-pressed="entry.done"
              :aria-label="entry.done ? `Mark ${entry.movement_name} not done` : `Mark ${entry.movement_name} done`"
              @click="toggleDone(entry)">
              {{ entry.done ? '✓' : '' }}
            </button>
            <span class="log-entry__name">{{ entry.movement_name }}</span>
          </div>

          <div class="actuals">
            <div class="actuals__sets">
              <span class="actuals__label">Sets</span>
              <div class="stepper">
                <button
                  type="button"
                  class="stepper__btn"
                  aria-label="Fewer sets"
                  @click="bumpSets(entry, -1)">
                  −
                </button>
                <span class="stepper__value">{{ entry.actual_sets ?? '—' }}</span>
                <button
                  type="button"
                  class="stepper__btn"
                  aria-label="More sets"
                  @click="bumpSets(entry, 1)">
                  +
                </button>
              </div>
            </div>

            <label class="actuals__field">
              <span class="actuals__label">Reps</span>
              <input
                class="actuals__input"
                type="text"
                inputmode="text"
                :value="entry.actual_reps ?? ''"
                placeholder="5, AMRAP, 30s"
                @change="saveReps(entry, ($event.target as HTMLInputElement).value)" />
            </label>

            <label class="actuals__field">
              <span class="actuals__label">Load</span>
              <input
                class="actuals__input"
                type="text"
                inputmode="text"
                :value="entry.actual_load ?? ''"
                placeholder="100lb, bodyweight"
                @change="saveLoad(entry, ($event.target as HTMLInputElement).value)" />
            </label>
          </div>

          <input
            class="log-entry__notes"
            type="text"
            :value="entry.notes"
            placeholder="Notes for this movement…"
            :aria-label="`Notes for ${entry.movement_name}`"
            @change="saveEntryNotes(entry, ($event.target as HTMLInputElement).value)" />
        </li>
      </ol>

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

      <div class="session__actions">
        <RouterLink
          class="btn btn--accent"
          :to="{ name: 'sessions' }">
          Done
        </RouterLink>
      </div>
    </template>
  </section>
</template>

<style scoped lang="scss">
.session {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.session__back {
  color: var(--text-muted);
  font-size: 0.9rem;
}

.session__status {
  padding: var(--space-4);
  text-align: center;
  color: var(--text-muted);

  &--error {
    color: #f87171;
  }
}

.session__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.session__title {
  margin: 0;
  font-size: 1.5rem;
}

.session__date {
  display: inline-block;
  margin-top: var(--space-1);
  color: var(--text-muted);
  font-size: 0.85rem;
  font-variant-numeric: tabular-nums;
}

.session__progress {
  flex-shrink: 0;
  padding: var(--space-1) var(--space-3);
  border-radius: 999px;
  background: var(--surface-raised);
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

.log {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.log-entry {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-3);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);

  &--done {
    border-color: var(--accent);
  }
}

.log-entry__top {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

// A large, one-handed-friendly checkbox — the most-tapped control on the screen.
.check {
  flex: 0 0 auto;
  width: var(--touch-target);
  height: var(--touch-target);
  display: grid;
  place-items: center;
  border: 2px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg);
  color: var(--accent-contrast);
  font-size: 1.4rem;
  line-height: 1;

  &--on {
    background: var(--accent);
    border-color: var(--accent);
  }
}

.log-entry__name {
  font-weight: 600;
  font-size: 1.05rem;
}

.log-entry--done .log-entry__name {
  color: var(--text-muted);
}

.actuals {
  display: grid;
  grid-template-columns: auto 1fr 1fr;
  gap: var(--space-2);
  align-items: end;
}

.actuals__sets {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.actuals__field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
}

.actuals__label {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.actuals__input {
  min-height: var(--touch-target);
  width: 100%;
  padding: 0 var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
}

.stepper {
  display: flex;
  align-items: center;
  gap: var(--space-1);
}

.stepper__btn {
  width: var(--touch-target);
  height: var(--touch-target);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  font-size: 1.3rem;
  line-height: 1;
}

.stepper__value {
  min-width: 2ch;
  text-align: center;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

.log-entry__notes {
  min-height: var(--touch-target);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
}

.section-title {
  margin: 0 0 var(--space-2);
  font-size: 1.05rem;
}

.meta {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
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
    min-height: 4.5rem;
    resize: vertical;
  }
}

.session__actions {
  display: flex;
  justify-content: flex-end;
}

.btn {
  min-height: var(--touch-target);
  display: inline-flex;
  align-items: center;
  padding: 0 var(--space-5);
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
}
</style>
