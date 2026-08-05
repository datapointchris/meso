<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { sessionsApi, setCount, isInProgress, type Session, type SessionFilter } from '@/api/sessions'
import type { Workout } from '@/api/workouts'
import { ApiError } from '@/api/client'
import DateField from '@/components/DateField.vue'
import StartSessionSheet from '@/components/StartSessionSheet.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

// The app's front door. Opening meso almost always means training, so the first thing
// on screen is the way into a session: resume the one already going, or start a new one.
// The history below it is the second reason to be here, not the first.
const router = useRouter()

const sessions = ref<Session[]>([])
const loading = ref(true)
const error = ref('')
const starting = ref(false)
const showStart = ref(false)
const pendingDelete = ref<Session | null>(null)
const deleting = ref(false)

// Server-side date filtering (the API owns the params), one definition shared with
// the CLI.
const filter = reactive<{ from: string; to: string }>({ from: '', to: '' })

async function load() {
  loading.value = true
  error.value = ''
  const query: SessionFilter = {}
  if (filter.from) query.from = filter.from
  if (filter.to) query.to = filter.to
  try {
    sessions.value = await sessionsApi.list(query)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load sessions'
  } finally {
    loading.value = false
  }
}

watch(() => [filter.from, filter.to], load)
onMounted(load)

const isEmpty = computed(() => !loading.value && !error.value && sessions.value.length === 0)

// The newest unfinished session, which is the one worth offering to pick up. Sessions
// come back newest first, so the first match is it.
const inProgress = computed(() => sessions.value.find(isInProgress) ?? null)

function openSession(s: Session) {
  router.push({ name: 'session-detail', params: { id: s.id } })
}

async function start(body: { workout_id?: number }) {
  if (starting.value) return
  starting.value = true
  try {
    const session = await sessionsApi.create(body)
    router.push({ name: 'session-detail', params: { id: session.id } })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to start a session.'
    starting.value = false
    showStart.value = false
  }
}

async function confirmDelete() {
  const target = pendingDelete.value
  if (!target || deleting.value) return
  deleting.value = true
  try {
    await sessionsApi.remove(target.id)
    sessions.value = sessions.value.filter((s) => s.id !== target.id)
    pendingDelete.value = null
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete the session.'
  } finally {
    deleting.value = false
  }
}

function sessionLabel(s: Session): string {
  return s.workout_name ?? 'Free-form session'
}

// What happened, rather than how much of the plan was hit. "2/5 done" reads as a score
// against a plan, and a session that went differently is not a session that failed.
function summary(s: Session): string {
  const movements = s.movements.length
  const sets = setCount(s)
  return `${movements} movement${movements === 1 ? '' : 's'} · ${sets} set${sets === 1 ? '' : 's'}`
}
</script>

<template>
  <section class="sessions">
    <header class="sessions__head">
      <h1 class="sessions__title">Sessions</h1>
    </header>

    <p
      v-if="error"
      class="sessions__status sessions__status--error">
      {{ error }}
    </p>

    <button
      v-if="inProgress"
      type="button"
      class="resume"
      @click="openSession(inProgress)">
      <span class="resume__label">▶ Resume session</span>
      <span class="resume__detail">{{ sessionLabel(inProgress) }} · {{ summary(inProgress) }}</span>
    </button>

    <button
      type="button"
      class="btn btn--accent start"
      :disabled="starting"
      @click="showStart = true">
      {{ starting ? 'Starting…' : '+ Start session' }}
    </button>

    <div class="sessions__filters">
      <div class="date-field">
        <span class="date-field__label">From</span>
        <DateField
          v-model="filter.from"
          label="From"
          clearable />
      </div>
      <div class="date-field">
        <span class="date-field__label">To</span>
        <DateField
          v-model="filter.to"
          label="To"
          clearable />
      </div>
    </div>

    <p
      v-if="loading"
      class="sessions__status">
      Loading…
    </p>
    <p
      v-else-if="isEmpty"
      class="sessions__status">
      No sessions yet. Tap “Start session” to begin one.
    </p>

    <ul
      v-else
      class="cards">
      <li
        v-for="s in sessions"
        :key="s.id"
        class="card">
        <div
          class="card__open"
          tabindex="0"
          role="button"
          @click="openSession(s)"
          @keydown.enter="openSession(s)">
          <div class="card__main">
            <span class="card__name">{{ sessionLabel(s) }}</span>
            <span class="card__date">{{ s.performed_on }}</span>
          </div>
          <div class="card__meta">
            <span class="card__count">{{ summary(s) }}</span>
            <span
              v-if="isInProgress(s)"
              class="tag tag--live">
              in progress
            </span>
            <span
              v-if="s.felt"
              class="tag">
              {{ s.felt }}
            </span>
          </div>
        </div>
        <button
          type="button"
          class="card__delete"
          :aria-label="`Delete the session on ${s.performed_on}`"
          @click="pendingDelete = s">
          ✕
        </button>
      </li>
    </ul>

    <StartSessionSheet
      v-if="showStart"
      :busy="starting"
      @programmed="(w: Workout) => start({ workout_id: w.id })"
      @free-form="start({})"
      @close="showStart = false" />

    <ConfirmDialog
      v-if="pendingDelete"
      title="Delete session"
      :message="`Delete the ${sessionLabel(pendingDelete).toLowerCase()} from ${pendingDelete.performed_on}? Everything logged in it goes too.`"
      confirm-label="Delete"
      danger
      :busy="deleting"
      @confirm="confirmDelete"
      @cancel="pendingDelete = null" />
  </section>
</template>

<style scoped lang="scss">
.sessions {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.sessions__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.sessions__title {
  margin: 0;
  font-size: 1.5rem;
}

// Sized above everything else on the screen: picking training back up is the one thing
// worth being able to hit without looking.
.resume {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--accent);
  border-radius: var(--radius);
  background: var(--surface-raised);
  color: var(--text);
  text-align: left;
}

.resume__label {
  font-weight: 700;
  font-size: 1.1rem;
  color: var(--accent);
}

.resume__detail {
  color: var(--text-muted);
  font-size: 0.85rem;
}

.start {
  min-height: calc(var(--touch-target) * 1.2);
  justify-content: center;
  font-size: 1.05rem;
}

.sessions__filters {
  display: flex;
  gap: var(--space-2);
}

.date-field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  flex: 1;
}

.date-field__label {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.sessions__status {
  padding: var(--space-4);
  color: var(--text-muted);
  text-align: center;

  &--error {
    color: var(--negative);
  }
}

.cards {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--space-2);
}

.card {
  display: flex;
  align-items: stretch;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
}

.card__open {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  cursor: pointer;

  &:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
}

.card__delete {
  flex-shrink: 0;
  width: var(--touch-target);
  border: none;
  border-left: 1px solid var(--border);
  background: transparent;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.card__main {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-2);
}

.card__name {
  font-weight: 600;
  font-size: 1.05rem;
}

.card__date {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
}

.card__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  font-size: 0.8rem;
  color: var(--text-muted);
}

.card__count {
  font-variant-numeric: tabular-nums;
}

.tag {
  padding: 2px var(--space-2);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 0.72rem;

  &--live {
    background: var(--accent);
    color: var(--accent-contrast);
  }
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

  &:disabled {
    opacity: 0.6;
  }
}

@media (min-width: 720px) {
  .cards {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
}
</style>
