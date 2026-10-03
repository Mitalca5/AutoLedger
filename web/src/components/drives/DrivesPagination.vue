<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-vue-next'
import { paginationPages as buildPaginationPages } from '@/utils/drives'

// Page range, page size buttons, page numbers and direct jump. Navigation is reported to the page.
const props = defineProps<{ page: number; limit: number; total: number; totalPages: number }>()
const emit = defineEmits<{ 'go-to-page': [page: number]; 'set-limit': [limit: number] }>()

function goToPage(target: number) {
  emit('go-to-page', target)
}

function setLimit(newLimit: number) {
  emit('set-limit', newLimit)
}

const jumpInput = ref<number | ''>('')
function applyJump() {
  if (jumpInput.value !== '') {
    goToPage(Number(jumpInput.value))
    jumpInput.value = ''
  }
}

const itemRangeStart = computed(() => {
  if (props.total === 0) return 0
  return (props.page - 1) * props.limit + 1
})

const itemRangeEnd = computed(() => {
  return Math.min(props.page * props.limit, props.total)
})

const paginationPages = computed(() => buildPaginationPages(props.page, props.totalPages))
</script>

<template>
  <div class="flex flex-col md:flex-row items-center justify-between gap-4 pt-4 border-t border-slate-800/80">
    <!-- Left: Range display & Page size buttons -->
    <div class="flex flex-wrap items-center gap-3 text-xs text-slate-400">
      <span>
        {{ $t('drives.drivesPagination.display') }} <strong class="text-white">{{ itemRangeStart }}</strong>–<strong class="text-white">{{ itemRangeEnd }}</strong> {{ $t('drives.drivesPagination.of') }} <strong class="text-white">{{ total }}</strong> {{ $t('drives.drivesPagination.drives') }}
      </span>
      <div class="flex items-center gap-1.5 border-l border-slate-800 pl-3">
        <span class="text-slate-400">{{ $t('drives.drivesPagination.perPage') }}</span>
        <button
          v-for="s in [20, 50, 100]"
          :key="s"
          @click="setLimit(s)"
          class="px-2 py-0.5 rounded-lg text-xs font-semibold transition-colors"
          :class="limit === s ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white bg-slate-800/60'"
        >
          {{ s }}
        </button>
      </div>
    </div>

    <!-- Center / Right: Numbered buttons + quick jump -->
    <div class="flex items-center gap-1.5 flex-wrap justify-center">
      <!-- First page -->
      <button
        @click="goToPage(1)"
        :disabled="page <= 1"
        :title="$t('drives.drivesPagination.firstPage')"
        class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
      >
        <ChevronsLeft class="w-4 h-4" />
      </button>
      <!-- Prev page -->
      <button
        @click="goToPage(page - 1)"
        :disabled="page <= 1"
        :title="$t('drives.drivesPagination.previousPage')"
        class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
      >
        <ChevronLeft class="w-4 h-4" />
      </button>

      <!-- Numbered pages with ellipses -->
      <template v-for="(p, idx) in paginationPages" :key="idx">
        <span v-if="p === '...'" class="px-1 text-xs text-slate-400 font-bold">...</span>
        <button
          v-else
          @click="goToPage(p as number)"
          class="min-w-[32px] h-8 px-2 rounded-xl text-xs font-semibold transition-all"
          :class="page === p ? 'bg-rose-600 text-white font-bold shadow-md shadow-rose-600/30' : 'bg-slate-900 border border-slate-800 text-slate-300 hover:text-white hover:border-slate-700'"
        >
          {{ p }}
        </button>
      </template>

      <!-- Next page -->
      <button
        @click="goToPage(page + 1)"
        :disabled="page >= totalPages"
        :title="$t('drives.drivesPagination.nextPage')"
        class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
      >
        <ChevronRight class="w-4 h-4" />
      </button>
      <!-- Last page -->
      <button
        @click="goToPage(totalPages)"
        :disabled="page >= totalPages"
        :title="$t('drives.drivesPagination.lastPage')"
        class="p-1.5 bg-slate-900 border border-slate-800 rounded-xl text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
      >
        <ChevronsRight class="w-4 h-4" />
      </button>

      <!-- Direct jump input -->
      <div v-if="totalPages > 1" class="flex items-center gap-1 ml-2 border-l border-slate-800 pl-2">
        <label for="drives-jump-page" class="text-xs text-slate-400">{{ $t('drives.drivesPagination.page') }}</label>
        <input
          id="drives-jump-page"
          type="number"
          min="1"
          :max="totalPages"
          v-model="jumpInput"
          @keydown.enter="applyJump"
          placeholder="N°"
          class="w-12 px-1.5 py-1 text-xs bg-slate-950 border border-slate-800 rounded-lg text-white text-center focus:border-rose-500 outline-none"
        />
        <button
          @click="applyJump"
          :disabled="!jumpInput"
          class="px-2 py-1 text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg disabled:opacity-30 disabled:cursor-not-allowed transition-colors font-medium"
        >
          {{ $t('drives.drivesPagination.go') }}
        </button>
      </div>
    </div>
  </div>
</template>
