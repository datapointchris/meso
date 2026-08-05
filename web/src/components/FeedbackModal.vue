<script setup lang="ts">
import { ref, onMounted, useTemplateRef } from 'vue'
import { useRoute } from 'vue-router'
import { feedbackApi } from '@/api/feedback'
import { ApiError } from '@/api/client'
import ModalShell from './ModalShell.vue'

// One field and a button. The whole value of this is that a thought had mid-session
// survives the moment, so anything that slows the capture — a category picker, a
// title, a severity — defeats the point. The route and the viewport are captured
// automatically because they are the triage context that is free now and
// unreconstructable later: most feedback about a mobile-first app is about layout,
// and "hard to read" is a different defect at 390px than at 1400px.
const emit = defineEmits<{ saved: []; close: [] }>()

const route = useRoute()
const body = ref('')
const saving = ref(false)
const saved = ref(false)
const error = ref('')

const field = useTemplateRef<HTMLTextAreaElement>('field')
onMounted(() => field.value?.focus())

async function save() {
  if (!body.value.trim()) {
    error.value = 'Say what happened.'
    return
  }
  saving.value = true
  error.value = ''
  try {
    await feedbackApi.capture({
      body: body.value.trim(),
      context_path: route.fullPath,
      viewport_width: window.innerWidth,
      viewport_height: window.innerHeight,
    })
    // Confirm before closing: at the gym on bad wifi, a sheet that vanishes gives no
    // evidence the thought actually landed.
    saved.value = true
    setTimeout(() => emit('saved'), 900)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to save that.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <ModalShell
    title="Feedback"
    @close="emit('close')">
    <p
      v-if="saved"
      class="modal__saved">
      Saved.
    </p>

    <!-- The form stays inside the slot rather than being the shell's body, because the
         saved state replaces it entirely. -->
    <form
      v-else
      class="feedback-form"
      @submit.prevent="save">
      <label class="field">
        <span class="field__label">
          What's on your mind?
          <em>{{ route.fullPath }}</em>
        </span>
        <textarea
          ref="field"
          v-model="body"
          class="field__input field__area"
          rows="4"
          placeholder="Anything — a papercut, a missing thing, an idea."
          required />
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
          {{ saving ? 'Saving…' : 'Send' }}
        </button>
      </div>
    </form>
  </ModalShell>
</template>

<style scoped lang="scss">
.feedback-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.modal__saved {
  margin: 0;
  padding: var(--space-6) var(--space-4) calc(var(--space-6) + var(--safe-bottom));
  color: var(--positive);
  font-weight: 600;
  text-align: center;
}

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
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
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
