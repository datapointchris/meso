<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import {
  metricsApi,
  slugifyMetricName,
  CATEGORY_LABELS,
  type MetricDefinition,
  type MetricCategory,
  type MetricDirection,
} from '@/api/measurements'
import { ApiError } from '@/api/client'
import { useConfirm } from '@/composables/useConfirm'
import ModalShell from './ModalShell.vue'

const { ask } = useConfirm()

// Defines or edits a metric — the tracked-stat vocabulary itself, as opposed to a
// reading against it (AddMeasurementModal). This is the UI half of `meso metrics
// define|edit|delete`; without it the only way to start tracking something new was
// the CLI.
//
// On edit the name is locked: it is the key measurements, cycle targets, and every
// CLI invocation reference. Renaming is delete + redefine, and a metric with
// readings can't be deleted (the API returns 409) — so the lock is real, not
// cosmetic.
const props = defineProps<{ metric?: MetricDefinition }>()
const emit = defineEmits<{ saved: [MetricDefinition]; deleted: [string]; close: [] }>()

const isEdit = computed(() => props.metric !== undefined)

const form = reactive({
  label: props.metric?.label ?? '',
  name: props.metric?.name ?? '',
  unit: props.metric?.unit ?? '',
  direction: (props.metric?.direction ?? 'higher_better') as MetricDirection,
  category: (props.metric?.category ?? 'strength') as MetricCategory,
  howToMeasure: props.metric?.how_to_measure ?? '',
})

// On a new metric the key tracks the label until the user edits the key themselves,
// at which point it stops following — a typed key is a deliberate choice.
const keyIsAuto = ref(!isEdit.value)
watch(
  () => form.label,
  (label) => {
    if (keyIsAuto.value) form.name = slugifyMetricName(label)
  },
)

const saving = ref(false)
const deleting = ref(false)
const error = ref('')

const CATEGORIES: MetricCategory[] = ['strength', 'cardio', 'mobility', 'body']

async function save() {
  if (!form.label.trim()) {
    error.value = 'Give the metric a name.'
    return
  }
  if (!form.name.trim()) {
    error.value = 'The key can’t be empty.'
    return
  }
  if (!form.unit.trim()) {
    error.value = 'Give the metric a unit (lb, seconds, cm, reps…).'
    return
  }
  saving.value = true
  error.value = ''
  try {
    const saved = props.metric
      ? await metricsApi.update(props.metric.name, {
          label: form.label.trim(),
          unit: form.unit.trim(),
          direction: form.direction,
          category: form.category,
          how_to_measure: form.howToMeasure.trim(),
        })
      : await metricsApi.define({
          name: form.name.trim(),
          label: form.label.trim(),
          unit: form.unit.trim(),
          direction: form.direction,
          category: form.category,
          how_to_measure: form.howToMeasure.trim(),
        })
    emit('saved', saved)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save the metric.'
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!props.metric) return
  const ok = await ask({
    title: 'Delete metric',
    message: `Delete “${props.metric.label}”? Metrics with recorded readings can’t be deleted.`,
    confirmLabel: 'Delete',
    danger: true,
  })
  if (!ok) return
  deleting.value = true
  error.value = ''
  try {
    await metricsApi.remove(props.metric.name)
    emit('deleted', props.metric.name)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete the metric.'
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <ModalShell
    :title="isEdit ? 'Edit metric' : 'New metric'"
    form
    @submit="save"
    @close="emit('close')">
    <label class="field">
      <span class="field__label">Name</span>
      <input
        v-model="form.label"
        class="field__input"
        type="text"
        placeholder="Row Machine 500m"
        required />
    </label>

    <label class="field">
      <span class="field__label">
        Key
        <em>{{ isEdit ? '(permanent)' : '(how the CLI addresses it)' }}</em>
      </span>
      <input
        v-model="form.name"
        class="field__input"
        type="text"
        placeholder="row-machine-500m"
        :disabled="isEdit"
        @input="keyIsAuto = false" />
    </label>

    <label class="field">
      <span class="field__label">Unit</span>
      <input
        v-model="form.unit"
        class="field__input"
        type="text"
        placeholder="seconds"
        required />
    </label>

    <label class="field">
      <span class="field__label">Category</span>
      <select
        v-model="form.category"
        class="field__input">
        <option
          v-for="c in CATEGORIES"
          :key="c"
          :value="c">
          {{ CATEGORY_LABELS[c] }}
        </option>
      </select>
    </label>

    <label class="field">
      <span class="field__label">
        Improvement
        <em>(which way is better)</em>
      </span>
      <select
        v-model="form.direction"
        class="field__input">
        <option value="higher_better">Higher is better</option>
        <option value="lower_better">Lower is better</option>
      </select>
    </label>

    <label class="field">
      <span class="field__label">
        How to measure
        <em>(what it is and how to take the reading)</em>
      </span>
      <textarea
        v-model="form.howToMeasure"
        class="field__input field__area"
        rows="4"
        placeholder="Single-leg heel raises to failure on flat ground, knee straight, full range. Count reps." />
    </label>

    <p
      v-if="error"
      class="modal__error">
      {{ error }}
    </p>

    <div class="modal__actions">
      <button
        v-if="isEdit"
        type="button"
        class="btn btn--danger"
        :disabled="deleting || saving"
        @click="remove">
        {{ deleting ? 'Deleting…' : 'Delete' }}
      </button>
      <span class="modal__spacer" />
      <button
        type="button"
        class="btn"
        @click="emit('close')">
        Cancel
      </button>
      <button
        type="submit"
        class="btn btn--accent"
        :disabled="saving || deleting">
        {{ saving ? 'Saving…' : 'Save' }}
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

  &:disabled {
    opacity: 0.6;
  }
}

.field__area {
  resize: vertical;
}

.modal__error {
  margin: 0;
  color: var(--negative);
  font-size: 0.9rem;
}

.modal__actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding-top: var(--space-2);
}

.modal__spacer {
  flex: 1;
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

  &--danger {
    border-color: var(--negative);
    color: var(--negative);
  }

  &:disabled {
    opacity: 0.6;
  }
}
</style>
