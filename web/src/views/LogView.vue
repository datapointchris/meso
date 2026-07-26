<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { logApi, type LogEntry, type LogFilter } from '@/api/log'
import { ApiError } from '@/api/client'
import { renderMarkdown } from '@/composables/useMarkdown'
import AddEditLogModal from '@/components/AddEditLogModal.vue'

// The training journal: dated markdown entries, newest first — the substrate Claude
// reviews when drafting the next cycle. Filter by date window or tag; add, edit, and
// delete entries in place.
const entries = ref<LogEntry[]>([])
const loading = ref(true)
const error = ref('')

// Server-side filtering (the API owns the params), one definition shared with the CLI.
const filter = reactive<{ from: string; to: string; tag: string }>({ from: '', to: '', tag: '' })

// null = closed; 'new' = creating; a LogEntry = editing that entry.
const editing = ref<LogEntry | 'new' | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  const query: LogFilter = {}
  if (filter.from) query.from = filter.from
  if (filter.to) query.to = filter.to
  if (filter.tag) query.tag = filter.tag.trim()
  try {
    entries.value = await logApi.list(query)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load the log'
  } finally {
    loading.value = false
  }
}

watch(() => [filter.from, filter.to, filter.tag], load)
onMounted(load)

const isEmpty = computed(() => !loading.value && !error.value && entries.value.length === 0)

function onSaved() {
  editing.value = null
  load()
}

async function remove(entry: LogEntry) {
  if (!window.confirm(`Delete the entry from ${entry.entry_date}? This cannot be undone.`)) return
  try {
    await logApi.remove(entry.id)
    entries.value = entries.value.filter((e) => e.id !== entry.id)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete the entry'
  }
}
</script>

<template>
  <section class="log">
    <header class="log__head">
      <h1 class="log__title">Log</h1>
      <button
        class="btn btn--accent"
        type="button"
        @click="editing = 'new'">
        New entry
      </button>
    </header>

    <div class="log__filters">
      <label class="field">
        <span class="field__label">From</span>
        <input
          v-model="filter.from"
          class="field__input"
          type="date" />
      </label>
      <label class="field">
        <span class="field__label">To</span>
        <input
          v-model="filter.to"
          class="field__input"
          type="date" />
      </label>
      <label class="field">
        <span class="field__label">Tag</span>
        <input
          v-model="filter.tag"
          class="field__input"
          type="text"
          placeholder="strength" />
      </label>
    </div>

    <p
      v-if="loading"
      class="log__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="log__status log__status--error">
      {{ error }}
    </p>
    <p
      v-else-if="isEmpty"
      class="log__status">
      No entries yet. Tap “New entry” to start the journal.
    </p>

    <ul
      v-else
      class="entries">
      <li
        v-for="entry in entries"
        :key="entry.id"
        class="entry">
        <div class="entry__head">
          <span class="entry__date">{{ entry.entry_date }}</span>
          <span
            v-if="entry.mood"
            class="entry__mood">
            {{ entry.mood }}
          </span>
          <span class="entry__spacer"></span>
          <button
            class="entry__action"
            type="button"
            aria-label="Edit entry"
            @click="editing = entry">
            Edit
          </button>
          <button
            class="entry__action entry__action--danger"
            type="button"
            aria-label="Delete entry"
            @click="remove(entry)">
            Delete
          </button>
        </div>

        <!-- eslint-disable-next-line vue/no-v-html -- first-party markdown, html disabled in the renderer -->
        <div
          class="entry__body prose"
          v-html="renderMarkdown(entry.body)"></div>

        <div
          v-if="entry.tags.length > 0"
          class="entry__tags">
          <span
            v-for="tag in entry.tags"
            :key="tag"
            class="tag">
            {{ tag }}
          </span>
        </div>
      </li>
    </ul>

    <AddEditLogModal
      v-if="editing"
      :entry="editing === 'new' ? undefined : editing"
      @saved="onSaved"
      @close="editing = null" />
  </section>
</template>

<style scoped lang="scss">
.log {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.log__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.log__title {
  margin: 0;
  font-size: 1.5rem;
}

.log__filters {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  flex: 1;
  min-width: 8rem;
}

.field__label {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.field__input {
  min-height: var(--touch-target);
  padding: 0 var(--space-2);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font: inherit;
}

.log__status {
  padding: var(--space-4);
  color: var(--text-muted);
  text-align: center;

  &--error {
    color: var(--negative);
  }
}

.entries {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--space-3);
}

.entry {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.entry__head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.entry__date {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.entry__mood {
  padding: 2px var(--space-2);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 0.72rem;
}

.entry__spacer {
  flex: 1;
}

.entry__action {
  min-height: var(--touch-target);
  padding: 0 var(--space-2);
  border: none;
  background: transparent;
  color: var(--text-muted);
  font: inherit;
  font-size: 0.85rem;

  &--danger {
    color: var(--negative);
  }
}

.entry__body {
  font-size: 0.95rem;
}

.entry__tags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1);
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
</style>
