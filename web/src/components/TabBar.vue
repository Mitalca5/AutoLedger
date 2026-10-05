<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, type Component } from 'vue'

export interface TabItem {
  key: string
  label: string
  icon?: Component
  /** Count or short text shown after the label. */
  badge?: string | number
  badgeTone?: 'neutral' | 'warning' | 'danger'
  /** Tab with nothing in it: still reachable, but greyed out. */
  muted?: boolean
}

const props = withDefaults(defineProps<{ tabs: TabItem[]; modelValue: string; label?: string; idPrefix?: string }>(), {
  label: undefined,
  idPrefix: 'tab',
})
const emit = defineEmits<{ 'update:modelValue': [key: string] }>()

const scroller = ref<HTMLElement | null>(null)
const canScrollLeft = ref(false)
const canScrollRight = ref(false)

function updateEdges() {
  const el = scroller.value
  if (!el) return
  canScrollLeft.value = el.scrollLeft > 1
  canScrollRight.value = el.scrollLeft + el.clientWidth < el.scrollWidth - 1
}

function select(key: string) {
  emit('update:modelValue', key)
}

async function onKeydown(event: KeyboardEvent) {
  const keys = props.tabs.map((t) => t.key)
  const index = keys.indexOf(props.modelValue)
  let next = -1
  if (event.key === 'ArrowRight') next = (index + 1) % keys.length
  else if (event.key === 'ArrowLeft') next = (index - 1 + keys.length) % keys.length
  else if (event.key === 'Home') next = 0
  else if (event.key === 'End') next = keys.length - 1
  if (next < 0) return
  event.preventDefault()
  select(keys[next])
  await nextTick()
  const button = scroller.value?.querySelector<HTMLElement>(`#${props.idPrefix}-${keys[next]}`)
  button?.focus()
  button?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
}

let observer: ResizeObserver | null = null
onMounted(() => {
  updateEdges()
  if (typeof ResizeObserver !== 'undefined' && scroller.value) {
    observer = new ResizeObserver(updateEdges)
    observer.observe(scroller.value)
  }
})
onBeforeUnmount(() => observer?.disconnect())

const badgeClass = (tone?: TabItem['badgeTone']) =>
  tone === 'danger'
    ? 'bg-rose-500 text-white'
    : tone === 'warning'
      ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
      : 'bg-slate-800 text-slate-300'
</script>

<template>
  <div class="relative border-b border-slate-800">
    <div ref="scroller" class="flex overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden" role="tablist" :aria-label="label" @scroll.passive="updateEdges">
      <button
        v-for="t in tabs"
        :id="`${idPrefix}-${t.key}`"
        :key="t.key"
        type="button"
        role="tab"
        :aria-selected="modelValue === t.key"
        :tabindex="modelValue === t.key ? 0 : -1"
        class="-mb-px flex min-h-11 shrink-0 items-center gap-2 whitespace-nowrap border-b-2 px-3.5 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:bg-slate-800/60 sm:px-4"
        :class="
          modelValue === t.key
            ? 'border-rose-500 text-white'
            : [t.muted ? 'text-slate-500' : 'text-slate-400', 'border-transparent hover:text-slate-200']
        "
        @click="select(t.key)"
        @keydown="onKeydown"
      >
        <component :is="t.icon" v-if="t.icon" class="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{{ t.label }}</span>
        <span v-if="t.badge !== undefined" class="rounded-full px-1.5 text-[11px] font-bold leading-5" :class="badgeClass(t.badgeTone)">{{ t.badge }}</span>
      </button>
    </div>
    <div v-if="canScrollLeft" class="pointer-events-none absolute inset-y-0 left-0 w-8 bg-gradient-to-r from-slate-950 to-transparent" aria-hidden="true" />
    <div v-if="canScrollRight" class="pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-slate-950 to-transparent" aria-hidden="true" />
  </div>
</template>
