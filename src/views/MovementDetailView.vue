<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  movementsApi,
  KIND_LABELS,
  RELATIONSHIP_KIND_LABELS,
  type Movement,
  type MovementMuscle,
  type RelatedMovement,
  type RelationshipKind,
} from '@/api/movements'
import { ApiError } from '@/api/client'
import { renderMarkdown } from '@/composables/useMarkdown'
import AddEditMovementModal from '@/components/AddEditMovementModal.vue'

const route = useRoute()
const router = useRouter()

const movement = ref<Movement | null>(null)
const loading = ref(true)
const error = ref('')
const showEdit = ref(false)

const id = computed(() => Number(route.params.id))

async function load() {
  loading.value = true
  error.value = ''
  try {
    movement.value = await movementsApi.get(id.value)
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      error.value = 'That movement no longer exists.'
    } else {
      error.value = e instanceof ApiError ? e.message : 'Failed to load movement.'
    }
  } finally {
    loading.value = false
  }
}

// The library for the relationship picker (all movements except this one).
const library = ref<Movement[]>([])
const pickRelated = ref<number | ''>('')
const pickKind = ref<RelationshipKind>('alternate')
const relError = ref('')
const relationshipKinds = Object.entries(RELATIONSHIP_KIND_LABELS) as [RelationshipKind, string][]

onMounted(async () => {
  await load()
  try {
    library.value = await movementsApi.list()
  } catch {
    // A failed library load only disables the relationship picker.
  }
})

const relatableMovements = computed(() => library.value.filter((m) => m.id !== id.value))

async function addRelationship() {
  if (!movement.value || pickRelated.value === '') return
  relError.value = ''
  try {
    movement.value = await movementsApi.addRelated(movement.value.id, {
      related_movement_id: pickRelated.value,
      relationship_kind: pickKind.value,
    })
    pickRelated.value = ''
  } catch (e) {
    relError.value = e instanceof ApiError ? e.message : 'Failed to add relationship.'
  }
}

async function removeRelationship(rel: RelatedMovement) {
  if (!movement.value) return
  relError.value = ''
  try {
    await movementsApi.removeRelated(movement.value.id, rel.id, rel.relationship_kind)
    await load()
  } catch (e) {
    relError.value = e instanceof ApiError ? e.message : 'Failed to remove relationship.'
  }
}

// Group muscles by role for the two-list display (primary first).
const primary = computed(() => (movement.value?.muscles ?? []).filter((m) => m.role === 'primary'))
const secondary = computed(() => (movement.value?.muscles ?? []).filter((m) => m.role === 'secondary'))

function muscleLabel(m: MovementMuscle): string {
  return `${m.muscle} · ${m.region}`
}

const defaultScheme = computed(() => {
  const m = movement.value
  if (!m) return ''
  const parts: string[] = []
  if (m.default_sets != null || m.default_reps) {
    parts.push(`${m.default_sets ?? '?'} × ${m.default_reps ?? '?'}`)
  }
  if (m.default_hold_seconds != null) parts.push(`hold ${m.default_hold_seconds}s`)
  return parts.join(' · ')
})

async function toggleFavorite() {
  if (!movement.value) return
  const next = !movement.value.favorite
  movement.value.favorite = next
  try {
    await movementsApi.update(movement.value.id, { favorite: next })
  } catch {
    movement.value.favorite = !next
  }
}

function onSaved(saved: Movement) {
  movement.value = saved
  showEdit.value = false
}

async function remove() {
  if (!movement.value) return
  if (!window.confirm(`Delete “${movement.value.name}”? This cannot be undone.`)) return
  try {
    await movementsApi.remove(movement.value.id)
    router.push({ name: 'movements' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to delete.'
  }
}
</script>

<template>
  <section class="detail">
    <RouterLink
      class="detail__back"
      :to="{ name: 'movements' }">
      ← Movements
    </RouterLink>

    <p
      v-if="loading"
      class="detail__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="detail__status detail__status--error">
      {{ error }}
    </p>

    <template v-else-if="movement">
      <header class="detail__head">
        <div>
          <h1 class="detail__title">{{ movement.name }}</h1>
          <p
            v-if="movement.sanskrit_name"
            class="detail__sanskrit">
            {{ movement.sanskrit_name }}
          </p>
          <span class="detail__kind">{{ KIND_LABELS[movement.movement_kind] }}</span>
        </div>
        <button
          type="button"
          class="detail__fav"
          :class="{ 'detail__fav--on': movement.favorite }"
          :aria-label="movement.favorite ? 'Remove from favorites' : 'Add to favorites'"
          @click="toggleFavorite">
          {{ movement.favorite ? '★' : '☆' }}
        </button>
      </header>

      <div class="chips-row">
        <span
          v-for="tag in movement.tags"
          :key="tag"
          class="tag">
          {{ tag }}
        </span>
      </div>

      <dl class="facts">
        <div v-if="movement.equipment.length">
          <dt>Equipment</dt>
          <dd>{{ movement.equipment.join(', ') }}</dd>
        </div>
        <div v-if="defaultScheme">
          <dt>Default</dt>
          <dd>{{ defaultScheme }}</dd>
        </div>
        <div v-if="movement.rating != null">
          <dt>Rating</dt>
          <dd>{{ movement.rating }}/5</dd>
        </div>
        <div v-if="movement.measurable_rom">
          <dt>ROM</dt>
          <dd>Tracked as a measurement</dd>
        </div>
      </dl>

      <section
        v-if="primary.length || secondary.length"
        class="muscles">
        <h2 class="section-title">Muscles</h2>
        <div class="muscles__cols">
          <div v-if="primary.length">
            <h3 class="muscles__role">Primary</h3>
            <ul>
              <li
                v-for="m in primary"
                :key="m.muscle">
                {{ muscleLabel(m) }}
              </li>
            </ul>
          </div>
          <div v-if="secondary.length">
            <h3 class="muscles__role">Secondary</h3>
            <ul>
              <li
                v-for="m in secondary"
                :key="m.muscle">
                {{ muscleLabel(m) }}
              </li>
            </ul>
          </div>
        </div>
      </section>

      <section
        v-if="movement.how_to"
        class="prose-block">
        <h2 class="section-title">How to</h2>
        <!-- eslint-disable-next-line vue/no-v-html -- first-party markdown, html disabled in the renderer -->
        <div
          class="prose"
          v-html="renderMarkdown(movement.how_to)" />
      </section>

      <section
        v-if="movement.form_cues"
        class="prose-block">
        <h2 class="section-title">Form cues</h2>
        <!-- eslint-disable-next-line vue/no-v-html -- first-party markdown, html disabled in the renderer -->
        <div
          class="prose"
          v-html="renderMarkdown(movement.form_cues)" />
      </section>

      <section
        v-if="movement.common_faults"
        class="prose-block">
        <h2 class="section-title">Common faults</h2>
        <!-- eslint-disable-next-line vue/no-v-html -- first-party markdown, html disabled in the renderer -->
        <div
          class="prose"
          v-html="renderMarkdown(movement.common_faults)" />
      </section>

      <section class="related">
        <h2 class="section-title">Related movements</h2>
        <ul
          v-if="movement.related.length"
          class="related__list">
          <li
            v-for="rel in movement.related"
            :key="`${rel.id}-${rel.relationship_kind}`"
            class="related__item">
            <RouterLink
              class="related__name"
              :to="{ name: 'movement-detail', params: { id: rel.id } }">
              {{ rel.name }}
            </RouterLink>
            <span class="related__kind">{{ RELATIONSHIP_KIND_LABELS[rel.relationship_kind] }}</span>
            <button
              type="button"
              class="related__remove"
              :aria-label="`Remove ${rel.name} relationship`"
              @click="removeRelationship(rel)">
              ✕
            </button>
          </li>
        </ul>
        <p
          v-else
          class="related__empty">
          None yet. Relate an alternate so the workout swap can offer it.
        </p>

        <div class="related__add">
          <select
            v-model="pickRelated"
            class="field__input"
            aria-label="Related movement">
            <option value="">Choose a movement…</option>
            <option
              v-for="m in relatableMovements"
              :key="m.id"
              :value="m.id">
              {{ m.name }}
            </option>
          </select>
          <select
            v-model="pickKind"
            class="field__input related__kind-select"
            aria-label="Relationship kind">
            <option
              v-for="[value, label] in relationshipKinds"
              :key="value"
              :value="value">
              {{ label }}
            </option>
          </select>
          <button
            type="button"
            class="btn"
            :disabled="pickRelated === ''"
            @click="addRelationship">
            Relate
          </button>
        </div>
        <p
          v-if="relError"
          class="related__error">
          {{ relError }}
        </p>
      </section>

      <p
        v-if="movement.source_name || movement.source_url"
        class="detail__source">
        Source:
        <a
          v-if="movement.source_url"
          :href="movement.source_url"
          target="_blank"
          rel="noopener">
          {{ movement.source_name || movement.source_url }}
        </a>
        <span v-else>{{ movement.source_name }}</span>
      </p>

      <div class="detail__actions">
        <button
          type="button"
          class="btn"
          @click="showEdit = true">
          Edit
        </button>
        <button
          type="button"
          class="btn btn--danger"
          @click="remove">
          Delete
        </button>
      </div>

      <AddEditMovementModal
        v-if="showEdit"
        :movement="movement"
        @saved="onSaved"
        @close="showEdit = false" />
    </template>
  </section>
</template>

<style scoped lang="scss">
.detail {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.detail__back {
  color: var(--text-muted);
  font-size: 0.9rem;
}

.detail__status {
  padding: var(--space-6);
  text-align: center;
  color: var(--text-muted);

  &--error {
    color: #f87171;
  }
}

.detail__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.detail__title {
  margin: 0;
  font-size: 1.6rem;
}

.detail__sanskrit {
  margin: var(--space-1) 0 0;
  color: var(--text-muted);
  font-style: italic;
}

.detail__kind {
  display: inline-block;
  margin-top: var(--space-2);
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.detail__fav {
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 1.6rem;
  line-height: 1;

  &--on {
    color: var(--accent);
  }
}

.chips-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.tag {
  padding: 2px var(--space-2);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text-muted);
  font-size: 0.75rem;
}

.facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: var(--space-3);
  margin: 0;
  padding: var(--space-3);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);

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

.muscles__cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-4);

  ul {
    margin: 0;
    padding-left: var(--space-4);
  }
}

.muscles__role {
  margin: 0 0 var(--space-1);
  font-size: 0.8rem;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.prose {
  color: var(--text);

  :deep(p) {
    margin: 0 0 var(--space-2);
  }

  :deep(ol),
  :deep(ul) {
    margin: 0 0 var(--space-2);
    padding-left: var(--space-6);
  }
}

.related {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.related__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.related__item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
}

.related__name {
  flex: 1 1 auto;
  color: var(--text);
  font-weight: 600;
}

.related__kind {
  flex: 0 0 auto;
  color: var(--text-muted);
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.related__remove {
  flex: 0 0 auto;
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  border: none;
  background: transparent;
  color: var(--text-muted);
}

.related__empty {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.related__add {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);

  .field__input {
    flex: 1 1 8rem;
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

.related__kind-select {
  flex: 0 0 auto;
}

.related__error {
  margin: 0;
  color: #f87171;
  font-size: 0.85rem;
}

.detail__source {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.85rem;
}

.detail__actions {
  display: flex;
  gap: var(--space-2);
}

.btn {
  min-height: var(--touch-target);
  padding: 0 var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface-raised);
  color: var(--text);
  font-weight: 600;

  &--danger {
    color: #f87171;
    border-color: #f87171;
    background: transparent;
  }
}
</style>
