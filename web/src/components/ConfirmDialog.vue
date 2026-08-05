<script setup lang="ts">
import ModalShell from './ModalShell.vue'

// Asking before something irreversible. This replaced window.confirm, which on a phone
// is a system alert whose buttons are wherever iOS decides and whose "OK" says nothing
// about what is being agreed to — a bad shape for a destructive action taken one-handed
// mid-workout.
withDefaults(
  defineProps<{
    title: string
    message: string
    /** The affirmative button's label. Say the verb, not "OK". */
    confirmLabel?: string
    /** Colours the affirmative button as destructive. */
    danger?: boolean
    busy?: boolean
  }>(),
  { confirmLabel: 'Confirm', danger: false, busy: false },
)
const emit = defineEmits<{ confirm: []; cancel: [] }>()
</script>

<template>
  <ModalShell
    :title="title"
    narrow
    @close="emit('cancel')">
    <p class="confirm__message">{{ message }}</p>
    <div class="confirm__actions">
      <button
        type="button"
        class="btn"
        @click="emit('cancel')">
        Cancel
      </button>
      <button
        type="button"
        class="btn"
        :class="danger ? 'btn--danger' : 'btn--accent'"
        :disabled="busy"
        @click="emit('confirm')">
        {{ busy ? 'Working…' : confirmLabel }}
      </button>
    </div>
  </ModalShell>
</template>

<style scoped lang="scss">
.confirm__message {
  margin: 0;
  color: var(--text);
  line-height: 1.5;
}

.confirm__actions {
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

  // Outlined rather than filled: --negative is a light red that would need a different
  // text colour per theme to stay readable, and the outline reads as destructive in both.
  &--danger {
    border-color: var(--negative);
    color: var(--negative);
  }

  &:disabled {
    opacity: 0.6;
  }
}
</style>
