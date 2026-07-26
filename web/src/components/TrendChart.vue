<script setup lang="ts">
import { computed } from 'vue'
import { changeVerdict, formatValue, type MetricTrend } from '@/api/measurements'

// A self-contained trend card: the metric's latest value, its direction-aware change
// badge, and an inline SVG line chart of the series. All colors come from theme CSS
// custom properties, so it tracks light/dark and the future design-style switcher.
//
// The card renders for a defined-but-unmeasured metric too — no chart, a "record the
// first reading" prompt instead. That is what makes the stats page a list of what
// *can* be tracked rather than only what already has history.
const props = defineProps<{ trend: MetricTrend }>()
const emit = defineEmits<{ record: []; edit: []; readings: [] }>()

// A fixed viewBox with default (aspect-preserving) scaling keeps the dots round while
// the SVG stretches to the card width via width:100%.
const W = 300
const H = 90
const PAD = 10

const verdict = computed(() => changeVerdict(props.trend))

// geometry maps the (oldest-first) points into viewBox space: x spread evenly, y
// scaled to the value range with padding, inverted (SVG y grows downward). A flat or
// single-point series pins to the vertical middle so it renders as a level line.
const geometry = computed(() => {
  const points = props.trend.points
  if (points.length === 0) return null

  const values = points.map((p) => p.value)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 1
  const stepX = points.length > 1 ? (W - 2 * PAD) / (points.length - 1) : 0

  const coords = points.map((p, i) => {
    const x = points.length > 1 ? PAD + i * stepX : W / 2
    const y = max === min ? H / 2 : H - PAD - ((p.value - min) / span) * (H - 2 * PAD)
    return { x, y }
  })

  const line = coords.map((c, i) => `${i === 0 ? 'M' : 'L'}${c.x.toFixed(1)},${c.y.toFixed(1)}`).join(' ')
  const area = `${line} L${coords[coords.length - 1].x.toFixed(1)},${H} L${coords[0].x.toFixed(1)},${H} Z`
  return { coords, line, area }
})

const changeLabel = computed(() => {
  const { change, unit } = props.trend
  if (change === null || change === 0) return 'no change'
  const sign = change > 0 ? '+' : '−'
  return `${sign}${formatValue(Math.abs(change))} ${unit}`
})
</script>

<template>
  <article
    class="trend"
    :class="verdict ? `trend--${verdict}` : ''">
    <header class="trend__head">
      <button
        class="trend__name"
        type="button"
        :aria-label="`Edit ${trend.label}`"
        @click="emit('edit')">
        {{ trend.label }}
      </button>
      <div class="trend__summary">
        <span class="trend__latest">
          {{ trend.latest !== null ? formatValue(trend.latest) : '—' }}
          <em class="trend__unit">{{ trend.unit }}</em>
        </span>
        <span
          v-if="verdict"
          class="trend__badge">
          {{ changeLabel }}
        </span>
      </div>
    </header>

    <svg
      v-if="geometry"
      class="trend__chart"
      :viewBox="`0 0 ${W} ${H}`"
      role="img"
      :aria-label="`${trend.label} trend, ${trend.count} readings`">
      <path
        class="trend__area"
        :d="geometry.area" />
      <path
        class="trend__line"
        :d="geometry.line" />
      <circle
        v-for="(c, i) in geometry.coords"
        :key="i"
        class="trend__dot"
        :cx="c.x"
        :cy="c.y"
        r="2.5" />
    </svg>

    <p
      v-else
      class="trend__empty">
      No readings yet
    </p>

    <footer class="trend__foot">
      <template v-if="geometry">
        <span>{{ trend.points[0]?.measured_on }}</span>
        <button
          class="trend__count"
          type="button"
          :aria-label="`View and edit ${trend.label} readings`"
          @click="emit('readings')">
          {{ trend.count }} reading{{ trend.count === 1 ? '' : 's' }}
        </button>
        <span>{{ trend.points[trend.points.length - 1]?.measured_on }}</span>
      </template>
      <span v-else />
      <button
        class="trend__record"
        type="button"
        @click="emit('record')">
        Record
      </button>
    </footer>
  </article>
</template>

<style scoped lang="scss">
.trend {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  // Verdict tint defaults to accent; overridden per-verdict below.
  --trend-color: var(--accent);
}

.trend--improved {
  --trend-color: var(--positive);
}

.trend--worse {
  --trend-color: var(--negative);
}

.trend--flat {
  --trend-color: var(--neutral);
}

.trend__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-2);
}

// The metric name doubles as the edit affordance — a button styled as the heading,
// so the card keeps one obvious tap target per action without a row of icons.
.trend__name {
  padding: 0;
  border: none;
  background: none;
  color: inherit;
  font: inherit;
  font-size: 0.95rem;
  font-weight: 600;
  text-align: left;
  word-break: break-word;
}

.trend__empty {
  margin: 0;
  padding: var(--space-3) 0;
  color: var(--text-muted);
  font-size: 0.85rem;
}

.trend__record {
  min-height: var(--touch-target);
  padding: 0 var(--space-3);
  margin: calc(var(--space-2) * -1) calc(var(--space-2) * -1) calc(var(--space-2) * -1) 0;
  border: none;
  background: none;
  color: var(--accent);
  font-size: 0.8rem;
  font-weight: 600;
}

.trend__summary {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  flex-shrink: 0;
}

.trend__latest {
  font-size: 1.1rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.trend__unit {
  font-style: normal;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--text-muted);
}

.trend__badge {
  padding: 1px var(--space-2);
  border-radius: 999px;
  background: color-mix(in srgb, var(--trend-color) 18%, transparent);
  color: var(--trend-color);
  font-size: 0.72rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.trend__chart {
  width: 100%;
  height: auto;
  overflow: visible;
}

.trend__area {
  fill: color-mix(in srgb, var(--trend-color) 14%, transparent);
  stroke: none;
}

.trend__line {
  fill: none;
  stroke: var(--trend-color);
  stroke-width: 2;
  stroke-linejoin: round;
  stroke-linecap: round;
}

.trend__dot {
  fill: var(--trend-color);
}

.trend__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  font-size: 0.72rem;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

// The reading count doubles as the way into the readings list, where a number that
// went in wrong gets corrected.
.trend__count {
  padding: 0;
  border: none;
  background: none;
  color: var(--text-muted);
  font: inherit;
  text-decoration: underline;
  text-decoration-style: dotted;
  text-underline-offset: 3px;
}
</style>
