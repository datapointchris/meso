<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  statsApi,
  metricsApi,
  CATEGORY_LABELS,
  type Stats,
  type MetricCategory,
  type MetricTrend,
  type MetricDefinition,
} from '@/api/measurements'
import { ApiError } from '@/api/client'
import TrendChart from '@/components/TrendChart.vue'
import AddMeasurementModal from '@/components/AddMeasurementModal.vue'
import AddEditMetricModal from '@/components/AddEditMetricModal.vue'
import MetricReadingsModal from '@/components/MetricReadingsModal.vue'
import MetricAboutModal from '@/components/MetricAboutModal.vue'

// The stats page: one GET assembles every *defined* metric's trend plus the library
// and session summaries. Trends group by category so related charts sit together.
// Unmeasured metrics come back with no points and render as an empty card — this
// page is the vocabulary as much as the history, so what can be recorded is visible
// without opening the record sheet first.
const stats = ref<Stats | null>(null)
const loading = ref(true)
const error = ref('')

// Recording is either null (closed) or the metric key to preselect; '' opens the
// sheet with no preselection, which is what the header button does.
const recording = ref<string | null>(null)

// Metric editing: null (closed), 'new', or the definition being edited.
const editingMetric = ref<MetricDefinition | 'new' | null>(null)

// The trend whose individual readings are open for correction, or null.
const viewingReadings = ref<MetricTrend | null>(null)

// The trend whose definition is being read — what it is and how to measure it.
const about = ref<MetricTrend | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    stats.value = await statsApi.get()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load stats'
  } finally {
    loading.value = false
  }
}

onMounted(load)

// The order categories render in — strength/cardio/mobility/body, matching the
// metric_definitions category set.
const CATEGORY_ORDER: MetricCategory[] = ['strength', 'cardio', 'mobility', 'body']

const metricsByCategory = computed(() => {
  const groups = new Map<MetricCategory, MetricTrend[]>()
  for (const trend of stats.value?.metrics ?? []) {
    const list = groups.get(trend.category) ?? []
    list.push(trend)
    groups.set(trend.category, list)
  }
  return CATEGORY_ORDER.filter((c) => groups.has(c)).map((c) => ({ category: c, trends: groups.get(c)! }))
})

const hasMetrics = computed(() => (stats.value?.metrics.length ?? 0) > 0)

// The tallest week bar sets the 100% height; each bar scales against it.
const maxWeekCount = computed(() => Math.max(1, ...(stats.value?.sessions.by_week ?? []).map((w) => w.count)))

function onSaved() {
  recording.value = null
  load()
}

// The trend card carries the metric key, not the definition; editing needs the full
// definition (unit/direction/category/how-to-measure), so fetch it by key.
async function openMetricEditor(metricName: string) {
  try {
    editingMetric.value = await metricsApi.get(metricName)
    about.value = null
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Failed to load the metric'
  }
}

function onMetricSaved() {
  editingMetric.value = null
  load()
}
</script>

<template>
  <section class="stats">
    <header class="stats__head">
      <h1 class="stats__title">Stats</h1>
      <div class="stats__actions">
        <button
          class="btn"
          type="button"
          @click="editingMetric = 'new'">
          New metric
        </button>
        <button
          class="btn btn--accent"
          type="button"
          @click="recording = ''">
          Record
        </button>
      </div>
    </header>

    <p
      v-if="loading"
      class="stats__status">
      Loading…
    </p>
    <p
      v-else-if="error"
      class="stats__status stats__status--error">
      {{ error }}
    </p>

    <template v-else-if="stats">
      <!-- Summary cards -->
      <div class="summary">
        <div class="summary__card">
          <span class="summary__value">{{ stats.library.total_movements }}</span>
          <span class="summary__label">Movements</span>
        </div>
        <div class="summary__card">
          <span class="summary__value">{{ stats.library.favorites }}</span>
          <span class="summary__label">Favorites</span>
        </div>
        <div class="summary__card">
          <span class="summary__value">{{ stats.sessions.total }}</span>
          <span class="summary__label">Sessions</span>
        </div>
        <div class="summary__card">
          <span class="summary__value">{{ stats.sessions.last_30_days }}</span>
          <span class="summary__label">Last 30 days</span>
        </div>
      </div>

      <!-- Session frequency by week -->
      <section
        v-if="stats.sessions.by_week.length > 0"
        class="panel">
        <h2 class="panel__title">Sessions by week</h2>
        <div class="weeks">
          <div
            v-for="w in stats.sessions.by_week"
            :key="w.week_start"
            class="weeks__col"
            :title="`Week of ${w.week_start}: ${w.count}`">
            <div class="weeks__bar-track">
              <div
                class="weeks__bar"
                :style="{ height: `${(w.count / maxWeekCount) * 100}%` }" />
            </div>
            <span class="weeks__count">{{ w.count }}</span>
            <span class="weeks__label">{{ w.week_start.slice(5) }}</span>
          </div>
        </div>
      </section>

      <!-- Movement mix by kind -->
      <section
        v-if="stats.library.by_kind.length > 0"
        class="panel">
        <h2 class="panel__title">Library by kind</h2>
        <ul class="mix">
          <li
            v-for="k in stats.library.by_kind"
            :key="k.kind"
            class="mix__item">
            <span class="mix__kind">{{ k.kind.replace('_', ' ') }}</span>
            <span class="mix__count">{{ k.count }}</span>
          </li>
        </ul>
      </section>

      <!-- Metric trends, grouped by category. Every defined metric gets a card,
           measured or not, so the page doubles as the list of what's trackable. -->
      <p
        v-if="!hasMetrics"
        class="stats__status">
        No metrics defined yet. Tap “New metric” to start tracking a lift, a time, or a range of motion.
      </p>

      <section
        v-for="group in metricsByCategory"
        :key="group.category"
        class="panel">
        <h2 class="panel__title">{{ CATEGORY_LABELS[group.category] }}</h2>
        <div class="trends">
          <TrendChart
            v-for="trend in group.trends"
            :key="trend.metric"
            :trend="trend"
            @record="recording = trend.metric"
            @about="about = trend"
            @readings="viewingReadings = trend" />
        </div>
      </section>
    </template>

    <AddMeasurementModal
      v-if="recording !== null"
      :metric="recording || undefined"
      @saved="onSaved"
      @close="recording = null" />

    <AddEditMetricModal
      v-if="editingMetric"
      :key="editingMetric === 'new' ? 'new' : editingMetric.name"
      :metric="editingMetric === 'new' ? undefined : editingMetric"
      @saved="onMetricSaved"
      @deleted="onMetricSaved"
      @close="editingMetric = null" />

    <MetricAboutModal
      v-if="about"
      :trend="about"
      @edit="openMetricEditor(about.metric)"
      @record="
        recording = about.metric
        about = null
      "
      @close="about = null" />

    <MetricReadingsModal
      v-if="viewingReadings"
      :metric="viewingReadings.metric"
      :label="viewingReadings.label"
      :unit="viewingReadings.unit"
      @changed="load"
      @close="viewingReadings = null" />
  </section>
</template>

<style scoped lang="scss">
.stats {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.stats__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stats__title {
  margin: 0;
  font-size: 1.5rem;
}

.stats__actions {
  display: flex;
  gap: var(--space-2);
}

.stats__status {
  padding: var(--space-4);
  color: var(--text-muted);
  text-align: center;

  &--error {
    color: var(--negative);
  }
}

.summary {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--space-2);
}

.summary__card {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: var(--space-3) var(--space-4);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.summary__value {
  font-size: 1.6rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.summary__label {
  font-size: 0.78rem;
  color: var(--text-muted);
}

.panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.panel__title {
  margin: 0;
  font-size: 1.05rem;
}

.weeks {
  display: flex;
  align-items: flex-end;
  gap: var(--space-1);
  padding: var(--space-3);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow-x: auto;
}

.weeks__col {
  flex: 1 0 2rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.weeks__bar-track {
  display: flex;
  align-items: flex-end;
  height: 80px;
  width: 100%;
}

.weeks__bar {
  width: 100%;
  min-height: 2px;
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
  background: var(--accent);
}

.weeks__count {
  font-size: 0.72rem;
  font-variant-numeric: tabular-nums;
}

.weeks__label {
  font-size: 0.65rem;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.mix {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}

.mix__item {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.mix__kind {
  text-transform: capitalize;
  font-size: 0.85rem;
}

.mix__count {
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.trends {
  display: grid;
  gap: var(--space-3);
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

@media (min-width: 720px) {
  .summary {
    grid-template-columns: repeat(4, 1fr);
  }

  .trends {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  }
}
</style>
