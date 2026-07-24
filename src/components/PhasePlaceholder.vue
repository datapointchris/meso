<script setup lang="ts">
// Temporary stand-in for a surface not yet built. Each of the five top-level
// views renders one in Phase 0; the real view replaces it in its own phase.
defineProps<{
  title: string
  phase: string
  description: string
}>()
</script>

<template>
  <section class="placeholder">
    <h1 class="placeholder__title">{{ title }}</h1>
    <p class="placeholder__desc">{{ description }}</p>
    <span class="placeholder__badge">Arrives in {{ phase }}</span>
  </section>
</template>

<style scoped lang="scss">
.placeholder {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-6);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.placeholder__title {
  margin: 0;
  font-size: 1.5rem;
}

.placeholder__desc {
  margin: 0;
  color: var(--text-muted);
}

.placeholder__badge {
  align-self: flex-start;
  padding: var(--space-1) var(--space-3);
  border-radius: 999px;
  background: var(--surface-raised);
  color: var(--accent);
  font-size: 0.8rem;
  font-weight: 600;
}
</style>
