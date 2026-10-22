<script setup lang="ts" generic="T extends string">
// One choice from a small closed vocabulary, laid out as a wrapping row of tap targets.
// A native <select> hands off to an OS picker: one tap to open it, a scroll, a tap to
// choose, and another to dismiss. Here every option is already on screen, so choosing
// is one tap. A vocabulary too long to lay out belongs in a search box instead, the
// shape MovementPicker takes.
//
// The pressed state is read from modelValue alone and never held in the DOM. A parent
// that treats a tap as an action rather than a selection binds a value no option
// carries, and gets an emit per tap with nothing left pressed.
const props = defineProps<{
  modelValue: T
  options: { value: T; label: string }[]
  label: string
}>()
const emit = defineEmits<{ 'update:modelValue': [T] }>()
</script>

<template>
  <div
    class="choicefield"
    role="group"
    :aria-label="props.label">
    <button
      v-for="option in props.options"
      :key="option.value"
      type="button"
      class="choicefield__option"
      :class="{ 'choicefield__option--selected': option.value === props.modelValue }"
      :aria-pressed="option.value === props.modelValue"
      @click="emit('update:modelValue', option.value)">
      {{ option.label }}
    </button>
  </div>
</template>

<style scoped lang="scss">
.choicefield {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.choicefield__option {
  min-height: var(--touch-target);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  font: inherit;

  &--selected {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
    font-weight: 700;
  }
}
</style>
