<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

// A date input sized for a thumb. The native <input type="date"> picker is a dense
// grid of small numerals — fine to read, hard to hit, and the reason "the calendar
// date chooser is crappy" came back as feedback. Rolling it by hand is what buys the
// touch targets and one consistent look across every date in the app; the app carries
// no component library to get this from.
//
// The value is an ISO "YYYY-MM-DD" string, matching the API's wire format for a DATE
// column. Dates are handled as plain calendar dates throughout — never Date objects
// crossing a timezone, which is what turns "2026-08-04" into the 3rd.
const props = withDefaults(defineProps<{ modelValue: string; label?: string; clearable?: boolean }>(), {
  label: 'Date',
  clearable: false,
})
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const open = ref(false)
const trigger = ref<HTMLButtonElement | null>(null)
const popover = ref<HTMLDivElement | null>(null)

const WEEKDAYS = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

interface CalendarDate {
  year: number
  month: number // 0-based, matching the MONTHS index
  day: number
}

function parseISO(iso: string): CalendarDate | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso)
  if (!match) return null
  return { year: Number(match[1]), month: Number(match[2]) - 1, day: Number(match[3]) }
}

function toISO(d: CalendarDate): string {
  return `${String(d.year).padStart(4, '0')}-${String(d.month + 1).padStart(2, '0')}-${String(d.day).padStart(2, '0')}`
}

function todayCalendar(): CalendarDate {
  const now = new Date()
  return { year: now.getFullYear(), month: now.getMonth(), day: now.getDate() }
}

function daysInMonth(year: number, month: number): number {
  return new Date(year, month + 1, 0).getDate()
}

// Monday-first offset: JS getDay() is Sunday-first, and a training week reads Mon–Sun.
function leadingBlanks(year: number, month: number): number {
  return (new Date(year, month, 1).getDay() + 6) % 7
}

// shiftDays moves a calendar date by n days via a local Date, which handles month and
// year rollover without a hand-written calendar.
function shiftDays(d: CalendarDate, n: number): CalendarDate {
  const js = new Date(d.year, d.month, d.day + n)
  return { year: js.getFullYear(), month: js.getMonth(), day: js.getDate() }
}

const selected = computed(() => parseISO(props.modelValue))
const today = todayCalendar()

// The month on screen, and the day the keyboard is on. Both start from the bound value
// so opening the picker lands where the value already is.
const cursor = ref<CalendarDate>(selected.value ?? today)
const viewYear = ref(cursor.value.year)
const viewMonth = ref(cursor.value.month)

watch(
  () => props.modelValue,
  (iso) => {
    const parsed = parseISO(iso)
    if (parsed) {
      cursor.value = parsed
      viewYear.value = parsed.year
      viewMonth.value = parsed.month
    }
  },
)

const displayValue = computed(() => {
  const d = selected.value
  if (!d) return ''
  return `${MONTHS[d.month].slice(0, 3)} ${d.day}, ${d.year}`
})

const grid = computed(() => {
  const cells: (CalendarDate | null)[] = Array.from({ length: leadingBlanks(viewYear.value, viewMonth.value) }, () => null)
  for (let day = 1; day <= daysInMonth(viewYear.value, viewMonth.value); day++) {
    cells.push({ year: viewYear.value, month: viewMonth.value, day })
  }
  return cells
})

function isSame(a: CalendarDate | null, b: CalendarDate | null): boolean {
  return a !== null && b !== null && a.year === b.year && a.month === b.month && a.day === b.day
}

function shiftMonth(delta: number) {
  const shifted = new Date(viewYear.value, viewMonth.value + delta, 1)
  viewYear.value = shifted.getFullYear()
  viewMonth.value = shifted.getMonth()
}

async function openPicker() {
  open.value = true
  cursor.value = selected.value ?? today
  viewYear.value = cursor.value.year
  viewMonth.value = cursor.value.month
  await nextTick()
  popover.value?.focus()
}

async function closePicker() {
  open.value = false
  await nextTick()
  trigger.value?.focus()
}

function choose(d: CalendarDate) {
  emit('update:modelValue', toISO(d))
  closePicker()
}

function clear() {
  emit('update:modelValue', '')
  closePicker()
}

// Arrow keys walk the calendar and pull the visible month along, so a date weeks away
// is reachable without hunting for the month arrows.
function onKeydown(event: KeyboardEvent) {
  const steps: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }
  const step = steps[event.key]
  if (step !== undefined) {
    event.preventDefault()
    cursor.value = shiftDays(cursor.value, step)
    viewYear.value = cursor.value.year
    viewMonth.value = cursor.value.month
    return
  }
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    choose(cursor.value)
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    closePicker()
  }
}
</script>

<template>
  <div class="datefield">
    <button
      ref="trigger"
      type="button"
      class="datefield__trigger"
      :aria-label="`${props.label}: ${displayValue || 'not set'}`"
      :aria-expanded="open"
      @click="open ? closePicker() : openPicker()">
      <span :class="{ datefield__placeholder: !displayValue }">{{ displayValue || 'Any date' }}</span>
      <span
        class="datefield__icon"
        aria-hidden="true">
        ▾
      </span>
    </button>

    <div
      v-if="open"
      class="datefield__backdrop"
      @click="closePicker" />

    <div
      v-if="open"
      ref="popover"
      class="datefield__popover"
      role="dialog"
      aria-modal="true"
      :aria-label="props.label"
      tabindex="-1"
      @keydown="onKeydown">
      <header class="datefield__head">
        <button
          type="button"
          class="datefield__nav"
          aria-label="Previous month"
          @click="shiftMonth(-1)">
          ‹
        </button>
        <span
          class="datefield__month"
          aria-live="polite">
          {{ MONTHS[viewMonth] }} {{ viewYear }}
        </span>
        <button
          type="button"
          class="datefield__nav"
          aria-label="Next month"
          @click="shiftMonth(1)">
          ›
        </button>
      </header>

      <div
        class="datefield__weekdays"
        aria-hidden="true">
        <span
          v-for="w in WEEKDAYS"
          :key="w">
          {{ w }}
        </span>
      </div>

      <div class="datefield__grid">
        <template
          v-for="(cell, i) in grid"
          :key="i">
          <span
            v-if="cell === null"
            class="datefield__blank" />
          <button
            v-else
            type="button"
            class="datefield__day"
            :class="{
              'datefield__day--selected': isSame(cell, selected),
              'datefield__day--today': isSame(cell, today),
              'datefield__day--cursor': isSame(cell, cursor),
            }"
            :aria-current="isSame(cell, today) ? 'date' : undefined"
            :aria-pressed="isSame(cell, selected)"
            @click="choose(cell)">
            {{ cell.day }}
          </button>
        </template>
      </div>

      <footer class="datefield__foot">
        <button
          type="button"
          class="datefield__action"
          @click="choose(today)">
          Today
        </button>
        <button
          v-if="props.clearable"
          type="button"
          class="datefield__action"
          @click="clear">
          Clear
        </button>
      </footer>
    </div>
  </div>
</template>

<style scoped lang="scss">
.datefield {
  position: relative;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.datefield__trigger {
  width: 100%;
  min-height: var(--touch-target);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg);
  color: var(--text);
  font: inherit;
  font-variant-numeric: tabular-nums;
  text-align: left;
}

.datefield__placeholder {
  color: var(--text-muted);
}

.datefield__icon {
  flex-shrink: 0;
  color: var(--text-muted);
}

// Catches the tap that dismisses the popover. Below the popover, above everything
// else — inside a modal the picker still has to sit on top of the form.
.datefield__backdrop {
  position: fixed;
  inset: 0;
  z-index: 60;
}

.datefield__popover {
  position: absolute;
  z-index: 61;
  top: calc(100% + var(--space-1));
  left: 0;
  min-width: 19rem;
  max-width: calc(100vw - var(--space-4));
  padding: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.35);

  &:focus {
    outline: none;
  }
}

.datefield__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
}

.datefield__month {
  font-weight: 600;
}

.datefield__nav {
  width: var(--touch-target);
  height: var(--touch-target);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  font-size: 1.3rem;
  line-height: 1;
}

.datefield__weekdays,
.datefield__grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: var(--space-1);
}

.datefield__weekdays {
  margin-bottom: var(--space-1);

  span {
    text-align: center;
    color: var(--text-muted);
    font-size: 0.7rem;
    text-transform: uppercase;
  }
}

// The whole point: a day is a real target with a readable numeral, not a 20px cell.
.datefield__day {
  min-width: var(--touch-target);
  min-height: var(--touch-target);
  display: grid;
  place-items: center;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  font-size: 1.05rem;
  font-variant-numeric: tabular-nums;

  &--today {
    border-color: var(--border);
    font-weight: 700;
  }

  &--cursor {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  &--selected {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
    font-weight: 700;
  }
}

.datefield__blank {
  min-height: var(--touch-target);
}

.datefield__foot {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-2);
  padding-top: var(--space-2);
  border-top: 1px solid var(--border);
}

.datefield__action {
  min-height: var(--touch-target);
  padding: 0 var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  font: inherit;
}
</style>
