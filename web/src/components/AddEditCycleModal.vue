<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { cyclesApi, CYCLE_STATUSES, type Cycle, type CycleWrite, type CycleStatus } from '@/api/cycles'
import { metricsApi, type MetricDefinition } from '@/api/measurements'
import { ApiError } from '@/api/client'
import DateField from '@/components/DateField.vue'
import ModalShell from './ModalShell.vue'

// When `cycle` is supplied the modal edits it (PUT); otherwise it creates (POST).
const props = defineProps<{ cycle?: Cycle }>()
const emit = defineEmits<{ saved: [Cycle]; close: [] }>()

const isEdit = computed(() => props.cycle !== undefined)

const form = reactive({
  name: props.cycle?.name ?? '',
  goal_summary: props.cycle?.goal_summary ?? '',
  status: (props.cycle?.status ?? 'planned') as CycleStatus,
  target_metric: props.cycle?.target_metric ?? '',
  target_value: props.cycle?.target_value ?? (null as number | null),
  start_date: props.cycle?.start_date ?? '',
  target_date: props.cycle?.target_date ?? '',
  notes: props.cycle?.notes ?? '',
})

// The metric vocabulary populates the target picker; a failed load just leaves the
// select empty (a cycle can have no numeric target).
const metrics = ref<MetricDefinition[]>([])
onMounted(async () => {
  try {
    metrics.value = await metricsApi.list()
  } catch {
    // non-fatal
  }
})

const saving = ref(false)
const error = ref('')

async function save() {
  if (!form.name.trim()) {
    error.value = 'Name is required.'
    return
  }
  saving.value = true
  error.value = ''

  const body: CycleWrite = {
    name: form.name.trim(),
    goal_summary: form.goal_summary,
    status: form.status,
    // Empty strings clear the nullable fields server-side.
    target_metric: form.target_metric || '',
    target_value: form.target_value === null || form.target_value === undefined ? null : Number(form.target_value),
    start_date: form.start_date || '',
    target_date: form.target_date || '',
    notes: form.notes,
  }

  try {
    const saved = props.cycle ? await cyclesApi.update(props.cycle.id, body) : await cyclesApi.create(body)
    emit('saved', saved)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <ModalShell
    :title="isEdit ? 'Edit cycle' : 'Add cycle'"
    form
    @submit="save"
    @close="emit('close')">
    <label class="field">
      <span class="field__label">Name</span>
      <input
        v-model="form.name"
        class="field__input"
        type="text"
        required />
    </label>

    <label class="field">
      <span class="field__label">Goal</span>
      <input
        v-model="form.goal_summary"
        class="field__input"
        type="text"
        placeholder="12-week run return" />
    </label>

    <label class="field field--inline">
      <span class="field__label">Status</span>
      <select
        v-model="form.status"
        class="field__input">
        <option
          v-for="s in CYCLE_STATUSES"
          :key="s"
          :value="s">
          {{ s }}
        </option>
      </select>
    </label>

    <label class="field">
      <span class="field__label">Target metric</span>
      <select
        v-model="form.target_metric"
        class="field__input">
        <option value="">No numeric target</option>
        <option
          v-for="m in metrics"
          :key="m.name"
          :value="m.name">
          {{ m.name }} ({{ m.unit }})
        </option>
      </select>
    </label>

    <label class="field field--inline">
      <span class="field__label">Target value</span>
      <input
        v-model.number="form.target_value"
        class="field__input"
        type="number"
        step="any"
        :disabled="!form.target_metric" />
    </label>

    <div class="field field--inline">
      <span class="field__label">Start date</span>
      <DateField
        v-model="form.start_date"
        label="Start date"
        clearable />
    </div>

    <div class="field field--inline">
      <span class="field__label">Target date</span>
      <DateField
        v-model="form.target_date"
        label="Target date"
        clearable />
    </div>

    <label class="field">
      <span class="field__label">Notes</span>
      <textarea
        v-model="form.notes"
        class="field__input field__area"
        rows="3" />
    </label>

    <p
      v-if="error"
      class="modal__error">
      {{ error }}
    </p>

    <div class="modal__actions">
      <button
        type="button"
        class="btn"
        @click="emit('close')">
        Cancel
      </button>
      <button
        type="submit"
        class="btn btn--accent"
        :disabled="saving">
        {{ saving ? 'Saving…' : isEdit ? 'Save' : 'Create' }}
      </button>
    </div>
  </ModalShell>
</template>

<style scoped lang="scss">
.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.field__label {
  font-size: 0.85rem;
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

  &:disabled {
    opacity: 0.5;
  }
}

.field__area {
  min-height: auto;
  resize: vertical;
}

.field--inline {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);

  .field__input {
    max-width: 14rem;
  }
}

.modal__error {
  margin: 0;
  color: #f87171;
  font-size: 0.9rem;
}

.modal__actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  padding-top: var(--space-2);
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

  &:disabled {
    opacity: 0.6;
  }
}
</style>
