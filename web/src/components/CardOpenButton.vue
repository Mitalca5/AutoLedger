<script setup lang="ts">
// The single control that opens a card: an invisible button stretched over it. The card stays a plain container, so the
// checkboxes and buttons inside it are not nested in another control; they sit above this button with `relative z-10`.
defineProps<{ label: string }>()
</script>

<template>
  <button type="button" class="absolute inset-0 rounded-[inherit] cursor-pointer" :aria-label="label" />
</template>
