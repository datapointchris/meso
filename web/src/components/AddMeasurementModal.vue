<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { metricsApi, measurementsApi, CATEGORY_LABELS, type Measurement, type MetricDefinition } from '@/api/measurements'
import { ApiError } from '@/api/client'
import DateField from '@/components/DateField.vue'
import ModalShell from './ModalShell.vue'

// Records a reading against an existing metric. Metrics are the vocabulary (seeded,
// or defined via the CLI/API); this modal is the fast in-app path to log a value.
const props = defineProps<{ metric?: string }>()
const emit = defineEmits<{ saved: [Measurement]; close: [] }>()

const metrics = ref<MetricDefinition[]>([])
const loadingMetrics = ref(true)

const form = reactive({
  metric: props.metric ?? '',
  value: null as number | null,
  measured_on: '',
  notes: '',
})

const saving = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    metrics.value = await metricsApi.list()
    if (!form.metric && metrics.value.length > 0) form.metric = metrics.value[0].name
  } catch {
    // A failed metric load leaves the picker empty; the message below explains.
  } finally {
    loadingMetrics.value = false
  }
})

async function save() {
  if (!form.metric) {
    error.value = 'Choose a metric.'
    return
  }
  if (form.value === null || Number.isNaN(form.value)) {
    error.value = 'Enter a value.'
    return
  }
  saving.value = true
  error.value = ''
  try {
    const saved = await measurementsApi.record({
      metric: form.metric,
      value: form.value,
      measured_on: form.measured_on || undefined,
      notes: form.notes || undefined,
    })
    emit('saved', saved)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to record.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <ModalShell
    title="Record measurement"
    form
    @submit="save"
    @close="emit('close')">
    <label class="field">
      <span class="field__label">Metric</span>
      <select
        v-model="form.metric"
        class="field__input"
        :disabled="loadingMetrics">
        <option
          v-for="m in metrics"
          :key="m.name"
          :value="m.name">
          {{ m.label }} ({{ m.unit }} · {{ CATEGORY_LABELS[m.category] }})
        </option>
      </select>
    </label>

    <p
      v-if="!loadingMetrics && metrics.length === 0"
      class="modal__hint">
      No metrics defined yet. Add one with “New metric” on the stats page (or
      <code>meso metrics define</code>
      ), then record against it.
    </p>

    <label class="field">
      <span class="field__label">Value</span>
      <input
        v-model.number="form.value"
        class="field__input"
        type="number"
        step="any"
        inputmode="decimal"
        required />
    </label>

    <div class="field">
      <span class="field__label">
        Date
        <em>(defaults to today)</em>
      </span>
      <DateField
        v-model="form.measured_on"
        label="Measured on"
        clearable />
    </div>

    <label class="field">
      <span class="field__label">Notes</span>
      <textarea
        v-model="form.notes"
        class="field__input field__area"
        rows="2" />
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
        :disabled="saving || metrics.length === 0">
        {{ saving ? 'Saving…' : 'Record' }}
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

  em {
    font-style: normal;
    opacity: 0.7;
  }
}

.field__input {
  min-height: var(--touch-target);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
}

.field__area {
  min-height: auto;
  resize: vertical;
}

.modal__hint {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted);

  code {
    padding: 1px 4px;
    border-radius: var(--radius-sm);
    background: var(--surface-raised);
    font-size: 0.85em;
  }
}

.modal__error {
  margin: 0;
  color: var(--negative);
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
