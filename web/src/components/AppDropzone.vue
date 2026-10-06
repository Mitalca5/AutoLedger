<script setup lang="ts">
import { ref, computed, watch, onUnmounted, useId } from 'vue'
import { UploadCloud, FileText, Image as ImageIcon, X, AlertCircle } from 'lucide-vue-next'
import { t } from '@/i18n'

const props = withDefaults(
  defineProps<{
    id?: string
    modelValue: File | null
    accept?: string
    maxSizeMb?: number
    label?: string
    helperText?: string
    disabled?: boolean
  }>(),
  {
    accept: '.pdf,image/png,image/jpeg,image/webp',
    maxSizeMb: 15,
    disabled: false,
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: File | null): void
  (e: 'error', message: string): void
}>()

const generatedId = useId()
const uniqueId = computed(() => props.id || generatedId)
const fileInputRef = ref<HTMLInputElement | null>(null)
const isDragging = ref(false)
const validationError = ref<string | null>(null)
const imagePreviewUrl = ref<string | null>(null)

const isImage = computed(() => {
  if (!props.modelValue) return false
  return props.modelValue.type.startsWith('image/')
})

const isPdf = computed(() => {
  if (!props.modelValue) return false
  return props.modelValue.type === 'application/pdf' || props.modelValue.name.toLowerCase().endsWith('.pdf')
})

function formatBytes(bytes: number): string {
  const sizes = t('shell.appDropzone.byteUnits').split(',')
  if (!bytes || bytes <= 0) return `0 ${sizes[0]}`
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

function updatePreview(file: File | null) {
  if (imagePreviewUrl.value) {
    URL.revokeObjectURL(imagePreviewUrl.value)
    imagePreviewUrl.value = null
  }
  if (file && file.type.startsWith('image/')) {
    imagePreviewUrl.value = URL.createObjectURL(file)
  }
}

watch(
  () => props.modelValue,
  (newFile) => {
    updatePreview(newFile)
    if (!newFile) {
      validationError.value = null
      if (fileInputRef.value) {
        fileInputRef.value.value = ''
      }
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  if (imagePreviewUrl.value) {
    URL.revokeObjectURL(imagePreviewUrl.value)
  }
})

function validateAndSetFile(file: File) {
  validationError.value = null

  // Check file size
  const maxBytes = props.maxSizeMb * 1024 * 1024
  if (file.size > maxBytes) {
    const msg = t('shell.appDropzone.tooLarge', { max: props.maxSizeMb, size: formatBytes(file.size) })
    validationError.value = msg
    emit('error', msg)
    return
  }

  // Check accepted formats
  const acceptedTypes = props.accept
    .split(',')
    .map((entry) => entry.trim().toLowerCase())
  const fileExt = `.${file.name.split('.').pop()?.toLowerCase() || ''}`
  const fileMime = file.type.toLowerCase()

  const isValid = acceptedTypes.some((type) => {
    if (type.startsWith('.')) {
      return fileExt === type
    }
    if (type.endsWith('/*')) {
      const baseType = type.replace('/*', '')
      return fileMime.startsWith(baseType)
    }
    return fileMime === type
  })

  if (!isValid) {
    const msg = t('shell.appDropzone.unsupportedFormat', { accept: props.accept })
    validationError.value = msg
    emit('error', msg)
    return
  }

  emit('update:modelValue', file)
}

function onFileSelect(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    validateAndSetFile(target.files[0])
  }
}

function onDragOver(e: DragEvent) {
  if (props.disabled) return
  e.preventDefault()
  isDragging.value = true
}

function onDragLeave(e: DragEvent) {
  if (props.disabled) return
  e.preventDefault()
  isDragging.value = false
}

function onDrop(e: DragEvent) {
  if (props.disabled) return
  e.preventDefault()
  isDragging.value = false
  if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
    validateAndSetFile(e.dataTransfer.files[0])
  }
}

function triggerFileInput() {
  if (props.disabled) return
  fileInputRef.value?.click()
}

function removeFile() {
  if (props.disabled) return
  emit('update:modelValue', null)
}
</script>

<template>
  <div class="space-y-2">
    <!-- Hidden native file input -->
    <label :for="uniqueId" class="sr-only">{{ label ?? $t('shell.appDropzone.defaultLabel') }}</label>
    <input
      :id="uniqueId"
      ref="fileInputRef"
      type="file"
      :accept="accept"
      :disabled="disabled"
      class="hidden"
      @change="onFileSelect"
    />

    <!-- Empty State / Dropzone -->
    <div
      v-if="!modelValue"
      v-clickable
      @click="triggerFileInput"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
      :class="[
        'relative border-2 border-dashed rounded-xl p-5 text-center transition-all cursor-pointer select-none flex flex-col items-center justify-center gap-2 group',
        isDragging
          ? 'border-indigo-400 bg-indigo-500/10 scale-[1.01]'
          : 'border-slate-700 hover:border-indigo-500/60 bg-slate-800/50 hover:bg-slate-800/80',
        disabled ? 'opacity-50 pointer-events-none cursor-not-allowed' : '',
      ]"
    >
      <div
        :class="[
          'w-11 h-11 rounded-xl flex items-center justify-center transition-colors',
          isDragging
            ? 'bg-indigo-500 text-white shadow-lg shadow-indigo-500/30'
            : 'bg-slate-800 text-slate-400 group-hover:text-indigo-400 group-hover:bg-slate-700/80',
        ]"
      >
        <UploadCloud class="w-6 h-6 animate-pulse transition-transform group-hover:-translate-y-0.5" />
      </div>
      <div>
        <p class="text-xs font-semibold text-slate-200 group-hover:text-white">
          {{ isDragging ? $t('shell.appDropzone.dropHere') : (label ?? $t('shell.appDropzone.defaultLabel')) }}
        </p>
        <p class="text-xs text-slate-400 mt-0.5">
          {{ helperText ?? $t('shell.appDropzone.defaultHelper', { max: maxSizeMb }) }}
        </p>
      </div>
    </div>

    <!-- Active File State -->
    <div
      v-else
      class="flex items-center justify-between p-3.5 bg-slate-800 border border-slate-700/80 rounded-xl gap-3"
    >
      <div class="flex items-center gap-3 min-w-0">
        <!-- Thumbnail preview or format icon -->
        <div class="w-12 h-12 rounded-lg bg-slate-900 border border-slate-700 flex items-center justify-center shrink-0 overflow-hidden">
          <img
            v-if="isImage && imagePreviewUrl"
            :src="imagePreviewUrl"
            :alt="modelValue.name"
            class="w-full h-full object-cover"
          />
          <FileText v-else-if="isPdf" class="w-6 h-6 text-rose-400" />
          <ImageIcon v-else-if="isImage" class="w-6 h-6 text-info-400" />
          <UploadCloud v-else class="w-6 h-6 text-slate-400" />
        </div>

        <div class="min-w-0">
          <p class="text-xs font-semibold text-white truncate" :title="modelValue.name">
            {{ modelValue.name }}
          </p>
          <div class="flex items-center gap-2 mt-0.5 text-xs text-slate-400">
            <span>{{ formatBytes(modelValue.size) }}</span>
            <span class="text-slate-400">•</span>
            <span class="uppercase text-xs font-mono px-1.5 py-0.5 rounded bg-slate-700/80 text-slate-300">
              {{ modelValue.name.split('.').pop() || 'FILE' }}
            </span>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-1 shrink-0">
        <button
          type="button"
          @click="triggerFileInput"
          :disabled="disabled"
          class="px-2.5 py-1 text-xs font-medium text-slate-300 hover:text-white bg-slate-700/70 hover:bg-slate-700 rounded-lg transition-colors"
        >
          {{ $t('shell.appDropzone.replace') }}
        </button>
        <button
          type="button"
          @click="removeFile"
          :disabled="disabled"
          class="tap p-1.5 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
          :title="$t('shell.appDropzone.removeThisFile')" :aria-label="$t('shell.appDropzone.removeThisFile')"
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Error Alert -->
    <div
      v-if="validationError"
      class="flex items-center gap-2 px-3 py-2 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-400 text-xs"
    >
      <AlertCircle class="w-4 h-4 shrink-0" />
      <span>{{ validationError }}</span>
    </div>
  </div>
</template>
