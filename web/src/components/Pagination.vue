<script setup>
import { computed } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  page: { type: Number, default: 1 },
  total: { type: Number, default: 0 },
  pageSize: { type: Number, default: 12 }
})
const emit = defineEmits(['change'])

const pageCount = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const list = computed(() => {
  const count = pageCount.value
  const current = props.page
  if (count <= 7) return Array.from({ length: count }, (_, i) => i + 1)
  const out = [1]
  if (current > 3) out.push('…')
  const start = Math.max(2, current - 1)
  const end = Math.min(count - 1, current + 1)
  for (let i = start; i <= end; i += 1) out.push(i)
  if (current < count - 2) out.push('…')
  out.push(count)
  return out
})

function go(p) {
  if (p < 1 || p > pageCount.value || p === props.page) return
  emit('change', p)
}
</script>

<template>
  <nav v-if="pageCount > 1" class="pagination" aria-label="分页">
    <button class="page-arrow" :disabled="page <= 1" @click="go(page - 1)" aria-label="上一页">
      <Icon name="arrowLeft" :size="15" />
    </button>
    <template v-for="(p, index) in list" :key="`${p}-${index}`">
      <span v-if="p === '…'" class="page-gap">…</span>
      <button v-else class="page-num" :class="{ active: p === page }" @click="go(p)">{{ p }}</button>
    </template>
    <button class="page-arrow" :disabled="page >= pageCount" @click="go(page + 1)" aria-label="下一页">
      <Icon name="arrowRight" :size="15" />
    </button>
  </nav>
</template>
