<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { workoutsApi, type Workout, type WorkoutFilter } from '@/api/workouts'
import { ApiError } from '@/api/client'
import AddEditWorkoutModal from '@/components/AddEditWorkoutModal.vue'

const router = useRouter()

const workouts = ref<Workout[]>([])
const loading = ref(true)
const error = ref('')
const showCreate = ref(false)

// Server-side filtering (the API owns the query params), so a change refetches
// rather than filtering a cached list — one filter definition shared with the CLI.
const filter = reactive<{ favorite: boolean; search: string }>({ favorite: false, search: '' })

async function load() {
  loading.value = true
  error.value = ''
  const query: WorkoutFilter = {}
  if (filter.favorite) query.favorite = true
  if (filter.search.trim()) query.search = filter.search.trim()
  try {
    workouts.value = await workoutsApi.list(query)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load workouts'
  } finally {
    loading.value = false
  }
}

watch(
  () => filter.favorite,
  () => load(),
)
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => filter.search,
  () => {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(load, 250)
  },
)

onMounted(load)

const isEmpty = computed(() => !loading.value && !error.value && workouts.value.length === 0)

function openWorkout(w: Workout) {
  router.push({ name: 'workout-detail', params: { id: w.id } })
}

async function toggleFavorite(w: Workout, event: Event) {
  event.stopPropagation()
  const next = !w.favorite
  w.favorite = next
  try {
    await workoutsApi.update(w.id, { favorite: next })
    if (filter.favorite && !next) {
      workouts.value = workouts.value.filter((x) => x.id !== w.id)
    }
  } catch {
    w.favorite = !next
  }
}

// A freshly created workout has no movements yet, so jump straight to its detail to
// compose it.
function onCreated(created: Workout) {
  showCreate.value = false
  router.push({ name: 'workout-detail', params: { id: created.id } })
}
</script>

<template>
  <section class="workouts">
    <header class="workouts__head">
      <h1 class="workouts__title">Workouts</h1>
      <button
        class="btn btn--accent"
        type="button"
        @click="showCreate = true">
        + Add
      </button>
    </header>

    <input
      v-model="filter.search"
      class="workouts__search"
      type="search"
      inputmode="search"
      placeholder="Search workouts…"
      aria-label="Search workouts" />

    <div class="workouts__filters">
      <button
        type="button"
        class="chip chip--fav"
        :class="{ 'chip--on': filter.favorite }"
        :aria-pressed="filter.favorite"
        @click="filter.favorite = !filter.favorite">
        ★ Favorites
      </button>
    </div>

    <p
      v-if="loading"
      class="workouts__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="workouts__status workouts__status--error">
      {{ error }}
    </p>
    <p
      v-else-if="isEmpty"
      class="workouts__status">
      No workouts yet. Add one to start composing.
    </p>

    <ul
      v-else
      class="cards">
      <li
        v-for="w in workouts"
        :key="w.id"
        class="card"
        tabindex="0"
        role="button"
        @click="openWorkout(w)"
        @keydown.enter="openWorkout(w)">
        <div class="card__main">
          <span class="card__name">{{ w.name }}</span>
          <span
            v-if="w.theme"
            class="card__theme">
            {{ w.theme }}
          </span>
        </div>
        <div class="card__meta">
          <span class="card__count">{{ w.movements.length }} movement{{ w.movements.length === 1 ? '' : 's' }}</span>
          <span
            v-for="tag in w.tags.slice(0, 3)"
            :key="tag"
            class="tag">
            {{ tag }}
          </span>
        </div>
        <button
          type="button"
          class="card__fav"
          :class="{ 'card__fav--on': w.favorite }"
          :aria-label="w.favorite ? 'Remove from favorites' : 'Add to favorites'"
          @click="toggleFavorite(w, $event)">
          {{ w.favorite ? '★' : '☆' }}
        </button>
      </li>
    </ul>

    <AddEditWorkoutModal
      v-if="showCreate"
      @saved="onCreated"
      @close="showCreate = false" />
  </section>
</template>

<style scoped lang="scss">
.workouts {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.workouts__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.workouts__title {
  margin: 0;
  font-size: 1.5rem;
}

.workouts__search {
  width: 100%;
  min-height: var(--touch-target);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  color: var(--text);
  font-size: 1rem;
}

.workouts__filters {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}

.chip {
  min-height: 36px;
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface);
  color: var(--text-muted);
  font-size: 0.85rem;

  &--on {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
    font-weight: 600;
  }
}

.workouts__status {
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
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  padding-right: calc(var(--touch-target) + var(--space-3));
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

.card__theme {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
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

.card__fav {
  position: absolute;
  top: 50%;
  right: var(--space-2);
  transform: translateY(-50%);
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 1.3rem;
  line-height: 1;

  &--on {
    color: var(--accent);
  }
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
}

@media (min-width: 720px) {
  .cards {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  }
}
</style>
