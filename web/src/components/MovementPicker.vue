<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { movementsApi, KIND_LABELS, type Movement } from '@/api/movements'
import { ApiError } from '@/api/client'

// One-tap movement selection, shared by the workout builder and the logging screen.
// The list itself is the control: a row is a button that picks. The pattern it
// replaced — filter box, native <select>, three prescription fields, an Add button —
// cost five interactions and an OS picker wheel per movement, which is unusable
// standing in a gym.
const props = withDefaults(defineProps<{ placeholder?: string; busy?: boolean }>(), {
  placeholder: 'Search the library…',
  busy: false,
})
const emit = defineEmits<{ pick: [Movement] }>()

const search = ref('')
const results = ref<Movement[]>([])
const loading = ref(true)
const error = ref('')

// Search runs server-side so the query is normalized there — punctuation and word
// order don't have to match the library's spelling. Filtering a cached array here
// would put the raw string back in charge.
async function load() {
  loading.value = true
  error.value = ''
  try {
    results.value = await movementsApi.list(search.value.trim() ? { search: search.value.trim() } : {})
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load movements.'
  } finally {
    loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(load, 250)
})

onMounted(load)

const isEmpty = computed(() => !loading.value && !error.value && results.value.length === 0)
</script>

<template>
  <div class="picker">
    <input
      v-model="search"
      class="picker__search"
      type="search"
      inputmode="search"
      :placeholder="props.placeholder"
      aria-label="Search the movement library" />

    <p
      v-if="loading"
      class="picker__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="picker__status picker__status--error">
      {{ error }}
    </p>
    <p
      v-else-if="isEmpty"
      class="picker__status">
      Nothing matches “{{ search.trim() }}”.
      <RouterLink :to="{ name: 'movements' }">Add it to the library</RouterLink>
      first.
    </p>

    <ul
      v-else
      class="picker__list">
      <li
        v-for="m in results"
        :key="m.id">
        <button
          type="button"
          class="picker__option"
          :disabled="props.busy"
          @click="emit('pick', m)">
          <span class="picker__name">{{ m.name }}</span>
          <span class="picker__kind">{{ KIND_LABELS[m.movement_kind] }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped lang="scss">
.picker {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  min-width: 0;
}

.picker__search {
  min-height: var(--touch-target);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
}

.picker__status {
  margin: 0;
  padding: var(--space-2) 0;
  color: var(--text-muted);
  font-size: 0.9rem;

  &--error {
    color: #f87171;
  }
}

// Capped so the list scrolls inside the panel rather than pushing the rest of the
// screen away — on a phone the search box has to stay reachable while scanning.
.picker__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  max-height: 42vh;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.picker__option {
  width: 100%;
  min-height: var(--touch-target);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  text-align: left;

  &:disabled {
    opacity: 0.5;
  }
}

.picker__name {
  font-weight: 600;
  min-width: 0;
}

.picker__kind {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
</style>
