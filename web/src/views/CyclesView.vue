<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { cyclesApi, cycleTargetSummary, CYCLE_STATUSES, type Cycle, type CycleFilter, type CycleStatus } from '@/api/cycles'
import { ApiError } from '@/api/client'
import AddEditCycleModal from '@/components/AddEditCycleModal.vue'

const router = useRouter()

const cycles = ref<Cycle[]>([])
const loading = ref(true)
const error = ref('')
const showCreate = ref(false)

// Server-side filtering (the API owns the query params), so a change refetches
// rather than filtering a cached list — one filter definition shared with the CLI.
const filter = reactive<{ status: CycleStatus | ''; search: string }>({ status: '', search: '' })

async function load() {
  loading.value = true
  error.value = ''
  const query: CycleFilter = {}
  if (filter.status) query.status = filter.status
  if (filter.search.trim()) query.search = filter.search.trim()
  try {
    cycles.value = await cyclesApi.list(query)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load cycles'
  } finally {
    loading.value = false
  }
}

watch(
  () => filter.status,
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

const isEmpty = computed(() => !loading.value && !error.value && cycles.value.length === 0)

function openCycle(c: Cycle) {
  router.push({ name: 'cycle-detail', params: { id: c.id } })
}

function toggleStatus(s: CycleStatus) {
  filter.status = filter.status === s ? '' : s
}

// A freshly created cycle has no workouts yet, so jump straight to its detail to
// compose the sequence.
function onCreated(created: Cycle) {
  showCreate.value = false
  router.push({ name: 'cycle-detail', params: { id: created.id } })
}
</script>

<template>
  <section class="cycles">
    <header class="cycles__head">
      <h1 class="cycles__title">Cycles</h1>
      <button
        class="btn btn--accent"
        type="button"
        @click="showCreate = true">
        + Add
      </button>
    </header>

    <input
      v-model="filter.search"
      class="cycles__search"
      type="search"
      inputmode="search"
      placeholder="Search cycles…"
      aria-label="Search cycles" />

    <div class="cycles__filters">
      <button
        v-for="s in CYCLE_STATUSES"
        :key="s"
        type="button"
        class="chip"
        :class="{ 'chip--on': filter.status === s }"
        :aria-pressed="filter.status === s"
        @click="toggleStatus(s)">
        {{ s }}
      </button>
    </div>

    <p
      v-if="loading"
      class="cycles__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="cycles__status cycles__status--error">
      {{ error }}
    </p>
    <p
      v-else-if="isEmpty"
      class="cycles__status">
      No cycles yet. Add one to start planning a block.
    </p>

    <ul
      v-else
      class="cards">
      <li
        v-for="c in cycles"
        :key="c.id"
        class="card"
        tabindex="0"
        role="button"
        @click="openCycle(c)"
        @keydown.enter="openCycle(c)">
        <div class="card__main">
          <span class="card__name">{{ c.name }}</span>
          <span
            class="card__status"
            :class="`card__status--${c.status}`">
            {{ c.status }}
          </span>
        </div>
        <p
          v-if="c.goal_summary"
          class="card__goal">
          {{ c.goal_summary }}
        </p>
        <div class="card__meta">
          <span class="card__count">{{ c.workouts.length }} workout{{ c.workouts.length === 1 ? '' : 's' }}</span>
          <span
            v-if="cycleTargetSummary(c)"
            class="tag">
            {{ cycleTargetSummary(c) }}
          </span>
          <span
            v-if="c.target_date"
            class="tag">
            by {{ c.target_date }}
          </span>
        </div>
      </li>
    </ul>

    <AddEditCycleModal
      v-if="showCreate"
      @saved="onCreated"
      @close="showCreate = false" />
  </section>
</template>

<style scoped lang="scss">
.cycles {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.cycles__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cycles__title {
  margin: 0;
  font-size: 1.5rem;
}

.cycles__search {
  width: 100%;
  min-height: var(--touch-target);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  color: var(--text);
  font-size: 1rem;
}

.cycles__filters {
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
  text-transform: capitalize;

  &--on {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
    font-weight: 600;
  }
}

.cycles__status {
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

// Status badge — colored by lifecycle so an active block reads at a glance.
.card__status {
  flex-shrink: 0;
  padding: 2px var(--space-2);
  border-radius: 999px;
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;

  &--active {
    background: var(--accent);
    color: var(--accent-contrast);
  }

  &--complete {
    color: var(--text-muted);
  }
}

.card__goal {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
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
