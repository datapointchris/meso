<script setup lang="ts">
import { CATEGORY_LABELS, type MetricTrend } from '@/api/measurements'
import { renderMarkdown } from '@/composables/useMarkdown'

// What a metric is and how to take the reading. Tapping a stat's name used to open
// the editor, which answers "what are this metric's settings" — never the question
// actually being asked, which is "what is this and what do I do to get a number".
// A metric name is not a movement name and never will be (there is no "Heel Raise
// Capacity Right" in the library), so there was nowhere to go and look it up.
//
// Edit stays reachable from here rather than from the card: changing a definition is
// rare, reading it is not, and the card only has room for one tap on the name.
const props = defineProps<{ trend: MetricTrend }>()
const emit = defineEmits<{ edit: []; record: []; close: [] }>()

const DIRECTION_LABELS: Record<string, string> = {
  higher_better: 'Higher is better',
  lower_better: 'Lower is better',
}

const facts = [
  { term: 'Unit', value: props.trend.unit },
  { term: 'Improvement', value: DIRECTION_LABELS[props.trend.direction] ?? props.trend.direction },
  { term: 'Category', value: CATEGORY_LABELS[props.trend.category] },
  { term: 'Key', value: props.trend.metric },
]
</script>

<template>
  <div
    class="overlay"
    @click.self="emit('close')">
    <div
      class="modal"
      role="dialog"
      aria-modal="true"
      :aria-label="`About ${trend.label}`">
      <header class="modal__head">
        <h2 class="modal__title">{{ trend.label }}</h2>
        <button
          class="modal__close"
          type="button"
          aria-label="Close"
          @click="emit('close')">
          ✕
        </button>
      </header>

      <div class="modal__body">
        <dl class="facts">
          <div
            v-for="fact in facts"
            :key="fact.term">
            <dt>{{ fact.term }}</dt>
            <dd>{{ fact.value }}</dd>
          </div>
        </dl>

        <section v-if="trend.how_to_measure">
          <h3 class="section-title">How to measure</h3>
          <!-- eslint-disable-next-line vue/no-v-html -- first-party markdown, html disabled in the renderer -->
          <div
            class="prose"
            v-html="renderMarkdown(trend.how_to_measure)" />
        </section>

        <p
          v-else
          class="modal__undocumented">
          No protocol recorded yet. Until there is one, every reading is only comparable to the others if you happen to remember how you
          took the last one — tap Edit to write down how this is measured.
        </p>

        <div class="modal__actions">
          <button
            type="button"
            class="btn"
            @click="emit('edit')">
            Edit
          </button>
          <span class="modal__spacer" />
          <button
            type="button"
            class="btn btn--accent"
            @click="emit('record')">
            Record
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
}

.modal {
  width: 100%;
  max-width: 32rem;
  max-height: 92dvh;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius) var(--radius) 0 0;
}

.modal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4);
  border-bottom: 1px solid var(--border);
}

.modal__title {
  margin: 0;
  font-size: 1.2rem;
}

.modal__close {
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 1.1rem;
}

.modal__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding: var(--space-4);
  padding-bottom: calc(var(--space-4) + var(--safe-bottom));
  overflow-y: auto;
}

.facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr));
  gap: var(--space-3);
  margin: 0;

  dt {
    color: var(--text-muted);
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }

  dd {
    margin: var(--space-1) 0 0;
  }
}

.section-title {
  margin: 0 0 var(--space-2);
  font-size: 1.05rem;
}

.modal__undocumented {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.modal__actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
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
}

@media (min-width: 720px) {
  .overlay {
    align-items: center;
  }

  .modal {
    border-radius: var(--radius);
  }
}
</style>
