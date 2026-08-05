<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { workoutsApi, type Workout } from '@/api/workouts'
import { ApiError } from '@/api/client'
import ModalShell from './ModalShell.vue'

// The one decision between opening the app and training: is this a workout that was
// planned, or is it whatever happens? Everything else — the date, a name — is filled in
// afterwards or not at all.
//
// Two steps rather than two buttons on the home screen, because picking a template needs
// a searchable list and a list of eighteen workouts is not a home screen.
const emit = defineEmits<{ programmed: [Workout]; freeForm: []; close: [] }>()
const props = withDefaults(defineProps<{ busy?: boolean }>(), { busy: false })

const choosing = ref(false)
const search = ref('')
const workouts = ref<Workout[]>([])
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    workouts.value = await workoutsApi.list(search.value.trim() ? { search: search.value.trim() } : {})
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load workouts.'
  } finally {
    loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(load, 250)
})

// Loaded up front so tapping "Programmed workout" shows the list rather than a spinner.
onMounted(load)

const isEmpty = computed(() => !loading.value && !error.value && workouts.value.length === 0)
</script>

<template>
  <ModalShell
    :title="choosing ? 'Pick a workout' : 'Start a session'"
    @close="emit('close')">
    <template v-if="!choosing">
      <button
        type="button"
        class="choice"
        :disabled="props.busy"
        @click="choosing = true">
        <span class="choice__label">Programmed workout</span>
        <span class="choice__hint">Copies its movements in as today's plan</span>
      </button>
      <button
        type="button"
        class="choice"
        :disabled="props.busy"
        @click="emit('freeForm')">
        <span class="choice__label">Free form</span>
        <span class="choice__hint">Starts empty — add movements as you do them</span>
      </button>
    </template>

    <template v-else>
      <input
        v-model="search"
        class="search"
        type="search"
        inputmode="search"
        placeholder="Search workouts…"
        aria-label="Search workouts" />

      <p
        v-if="loading"
        class="status">
        Loading…
      </p>
      <p
        v-else-if="error"
        class="status status--error">
        {{ error }}
      </p>
      <p
        v-else-if="isEmpty"
        class="status">
        Nothing matches “{{ search.trim() }}”.
      </p>

      <ul
        v-else
        class="list">
        <li
          v-for="w in workouts"
          :key="w.id">
          <button
            type="button"
            class="option"
            :disabled="props.busy"
            @click="emit('programmed', w)">
            <span class="option__name">{{ w.name }}</span>
            <span
              v-if="w.theme"
              class="option__theme">
              {{ w.theme }}
            </span>
          </button>
        </li>
      </ul>

      <button
        type="button"
        class="back"
        @click="choosing = false">
        ← Back
      </button>
    </template>
  </ModalShell>
</template>

<style scoped lang="scss">
.choice {
  width: 100%;
  min-height: calc(var(--touch-target) * 1.4);
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 2px;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface-raised);
  color: var(--text);
  text-align: left;

  &:disabled {
    opacity: 0.6;
  }
}

.choice__label {
  font-weight: 600;
  font-size: 1.05rem;
}

.choice__hint {
  color: var(--text-muted);
  font-size: 0.82rem;
}

.search {
  min-height: var(--touch-target);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
}

.status {
  margin: 0;
  padding: var(--space-2) 0;
  color: var(--text-muted);
  font-size: 0.9rem;

  &--error {
    color: var(--negative);
  }
}

// Capped so the list scrolls inside the sheet and the search box stays reachable.
.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  max-height: 46vh;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.option {
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

.option__name {
  font-weight: 600;
  min-width: 0;
}

.option__theme {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.back {
  align-self: flex-start;
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--accent);
  font: inherit;
  padding: 0;
}
</style>
