<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { sessionsApi, doneCount, type Session, type SessionFilter } from '@/api/sessions'
import { ApiError } from '@/api/client'

// The session history: past and in-progress sessions, newest first, so a session
// started earlier can be resumed and completed ones reviewed. Sessions are started
// from a workout (Workouts → a workout → Start session), not created here.
const router = useRouter()

const sessions = ref<Session[]>([])
const loading = ref(true)
const error = ref('')

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

function openSession(s: Session) {
  router.push({ name: 'session-detail', params: { id: s.id } })
}
</script>

<template>
  <section class="sessions">
    <header class="sessions__head">
      <h1 class="sessions__title">Sessions</h1>
      <RouterLink
        class="btn"
        :to="{ name: 'workouts' }">
        Start one
      </RouterLink>
    </header>

    <div class="sessions__filters">
      <label class="date-field">
        <span class="date-field__label">From</span>
        <input
          v-model="filter.from"
          class="date-field__input"
          type="date" />
      </label>
      <label class="date-field">
        <span class="date-field__label">To</span>
        <input
          v-model="filter.to"
          class="date-field__input"
          type="date" />
      </label>
    </div>

    <p
      v-if="loading"
      class="sessions__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="sessions__status sessions__status--error">
      {{ error }}
    </p>
    <p
      v-else-if="isEmpty"
      class="sessions__status">
      No sessions yet. Open a workout and tap “Start session” to log one.
    </p>

    <ul
      v-else
      class="cards">
      <li
        v-for="s in sessions"
        :key="s.id"
        class="card"
        tabindex="0"
        role="button"
        @click="openSession(s)"
        @keydown.enter="openSession(s)">
        <div class="card__main">
          <span class="card__name">{{ s.workout_name ?? 'Ad-hoc session' }}</span>
          <span class="card__date">{{ s.performed_on }}</span>
        </div>
        <div class="card__meta">
          <span class="card__count">{{ doneCount(s) }} / {{ s.movements.length }} done</span>
          <span
            v-if="s.felt"
            class="tag">
            {{ s.felt }}
          </span>
        </div>
      </li>
    </ul>
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
}

.sessions__title {
  margin: 0;
  font-size: 1.5rem;
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

.date-field__input {
  min-height: var(--touch-target);
  padding: 0 var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.sessions__status {
  padding: var(--space-4);
  color: var(--text-muted);
  text-align: center;

  &--error {
    color: #f87171;
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
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  cursor: pointer;

  &:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
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
}

@media (min-width: 720px) {
  .cards {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
}
</style>
