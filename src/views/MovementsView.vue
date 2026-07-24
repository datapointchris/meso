<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { movementsApi, primaryMuscles, KIND_LABELS, type Movement, type MovementKind, type MovementFilter } from '@/api/movements'
import { ApiError } from '@/api/client'
import AddEditMovementModal from '@/components/AddEditMovementModal.vue'

const router = useRouter()

const movements = ref<Movement[]>([])
const loading = ref(true)
const error = ref('')
const showCreate = ref(false)

// The active filter. Server-side filtering (the API owns the query params), so a
// change here refetches rather than filtering a cached list — one definition of
// each filter, shared with the CLI.
const filter = reactive<{ kind: MovementKind | ''; favorite: boolean; search: string }>({
  kind: '',
  favorite: false,
  search: '',
})

const kinds: { value: MovementKind | ''; label: string }[] = [
  { value: '', label: 'All' },
  { value: 'exercise', label: 'Exercises' },
  { value: 'stretch', label: 'Stretches' },
  { value: 'yoga_pose', label: 'Poses' },
]

async function load() {
  loading.value = true
  error.value = ''
  const query: MovementFilter = {}
  if (filter.kind) query.kind = filter.kind
  if (filter.favorite) query.favorite = true
  if (filter.search.trim()) query.search = filter.search.trim()
  try {
    movements.value = await movementsApi.list(query)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load movements'
  } finally {
    loading.value = false
  }
}

// Refetch on kind/favorite immediately; debounce the free-text search so typing
// doesn't fire a request per keystroke.
watch(
  () => [filter.kind, filter.favorite],
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

const isEmpty = computed(() => !loading.value && !error.value && movements.value.length === 0)

function openMovement(m: Movement) {
  router.push({ name: 'movement-detail', params: { id: m.id } })
}

// Toggle favorite inline without leaving the list. Optimistic: flip locally, then
// reconcile with the server (revert on failure).
async function toggleFavorite(m: Movement, event: Event) {
  event.stopPropagation()
  const next = !m.favorite
  m.favorite = next
  try {
    await movementsApi.update(m.id, { favorite: next })
    // If the favorites filter is on and we just un-favorited, drop it from view.
    if (filter.favorite && !next) {
      movements.value = movements.value.filter((x) => x.id !== m.id)
    }
  } catch {
    m.favorite = !next // revert
  }
}

function onCreated() {
  showCreate.value = false
  load()
}
</script>

<template>
  <section class="movements">
    <header class="movements__head">
      <h1 class="movements__title">Movements</h1>
      <button
        class="btn btn--accent"
        type="button"
        @click="showCreate = true">
        + Add
      </button>
    </header>

    <input
      v-model="filter.search"
      class="movements__search"
      type="search"
      inputmode="search"
      placeholder="Search movements…"
      aria-label="Search movements" />

    <div class="movements__filters">
      <div
        class="chips"
        role="group"
        aria-label="Filter by kind">
        <button
          v-for="k in kinds"
          :key="k.value"
          type="button"
          class="chip"
          :class="{ 'chip--on': filter.kind === k.value }"
          @click="filter.kind = k.value">
          {{ k.label }}
        </button>
      </div>
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
      class="movements__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="movements__status movements__status--error">
      {{ error }}
    </p>
    <p
      v-else-if="isEmpty"
      class="movements__status">
      No movements match. Try clearing filters or add one.
    </p>

    <ul
      v-else
      class="cards">
      <li
        v-for="m in movements"
        :key="m.id"
        class="card"
        tabindex="0"
        role="button"
        @click="openMovement(m)"
        @keydown.enter="openMovement(m)">
        <div class="card__main">
          <span class="card__name">{{ m.name }}</span>
          <span class="card__kind">{{ KIND_LABELS[m.movement_kind] }}</span>
        </div>
        <div
          v-if="primaryMuscles(m).length || m.tags.length"
          class="card__meta">
          <span
            v-if="primaryMuscles(m).length"
            class="card__muscles">
            {{ primaryMuscles(m).join(', ') }}
          </span>
          <span
            v-for="tag in m.tags.slice(0, 3)"
            :key="tag"
            class="tag">
            {{ tag }}
          </span>
        </div>
        <button
          type="button"
          class="card__fav"
          :class="{ 'card__fav--on': m.favorite }"
          :aria-label="m.favorite ? 'Remove from favorites' : 'Add to favorites'"
          @click="toggleFavorite(m, $event)">
          {{ m.favorite ? '★' : '☆' }}
        </button>
      </li>
    </ul>

    <AddEditMovementModal
      v-if="showCreate"
      @saved="onCreated"
      @close="showCreate = false" />
  </section>
</template>

<style scoped lang="scss">
.movements {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.movements__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.movements__title {
  margin: 0;
  font-size: 1.5rem;
}

.movements__search {
  width: 100%;
  min-height: var(--touch-target);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  color: var(--text);
  font-size: 1rem;
}

.movements__filters {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
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

  &--fav {
    margin-left: auto;
  }
}

.movements__status {
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

.card__kind {
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
