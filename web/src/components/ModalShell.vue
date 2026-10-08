<script setup lang="ts">
import { computed, type Component } from 'vue'
import { X } from 'lucide-vue-next'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { modalFooterClass, modalLayerClass, modalWidthClass, type ModalSize } from '@/utils/modal'

// The frame every modal shares: backdrop on its own layer, a panel of one of three widths, a header with its title, an
// optional icon and the close button, a scrolling body and an optional footer. A click on the backdrop and Escape close it;
// v-dialog on the panel keeps the focus inside and protects what has been typed. Extra attributes land on the panel.
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    title?: string
    icon?: Component
    iconClass?: string
    size?: ModalSize
    nested?: boolean
    bodyClass?: string
    footerClass?: string
  }>(),
  { size: 'md', nested: false, bodyClass: 'space-y-4', footerClass: 'justify-end gap-2' },
)
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))

const layer = computed(() => modalLayerClass(props.nested))
const width = computed(() => modalWidthClass(props.size))
const footer = computed(() => modalFooterClass(props.footerClass))
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    :class="layer"
    @click.self="open = false"
  >
    <div v-dialog v-bind="$attrs" class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto" :class="width">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between gap-3 shrink-0 bg-slate-900/95">
        <slot name="title">
          <h3 class="text-base font-bold text-white flex items-center gap-2 min-w-0">
            <component :is="icon" v-if="icon" class="w-5 h-5 shrink-0" :class="iconClass" aria-hidden="true" />
            {{ title }}
          </h3>
        </slot>
        <div class="flex items-center gap-1.5 shrink-0">
          <slot name="actions" />
          <button type="button" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')" @click="open = false">
            <X class="w-5 h-5" aria-hidden="true" />
          </button>
        </div>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain" :class="bodyClass">
        <slot />
      </div>

      <div v-if="$slots.footer" :class="footer">
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>
