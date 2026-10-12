<script setup lang="ts">
// The text alternative of a chart: the same figures as a table, read by screen readers. A table is not clipped by
// sr-only (its width ignores the 1px), so the class goes on a wrapper.
defineProps<{ caption: string; headers: string[]; rows: (string | number)[][] }>()
</script>

<template>
  <div class="sr-only">
    <table>
      <caption>{{ caption }}</caption>
      <thead>
        <tr><th v-for="h in headers" :key="h" scope="col">{{ h }}</th></tr>
      </thead>
      <tbody>
        <tr v-for="(row, i) in rows" :key="i">
          <th scope="row">{{ row[0] }}</th>
          <td v-for="(cell, j) in row.slice(1)" :key="j">{{ cell }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
