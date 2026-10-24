<script setup>
import { reactive, watch } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  activeId: { type: [Number, String], default: null },
  depth: { type: Number, default: 0 },
  ancestorPrivate: { type: Boolean, default: false }
})

const openMap = reactive({})
watch(
  () => props.nodes,
  (nodes) => {
    nodes.forEach((node) => {
      if (openMap[node.id] === undefined) openMap[node.id] = true
    })
  },
  { immediate: true }
)
</script>

<template>
  <ul class="doc-tree" :class="{ 'doc-tree--nested': depth > 0 }">
    <li v-for="node in nodes" :key="node.id" class="doc-tree-item">
      <div class="doc-tree-row" :class="{ active: String(activeId) === String(node.id) }">
        <button
          v-if="node.children?.length"
          class="tree-toggle"
          :class="{ open: openMap[node.id] }"
          type="button"
          aria-label="展开或收起"
          @click="openMap[node.id] = !openMap[node.id]"
        >
          <Icon name="chevronRight" :size="13" />
        </button>
        <span v-else class="tree-dot"></span>
        <RouterLink :to="`/docs/${node.id}`" class="tree-link"><span>{{ node.title }}</span><Icon v-if="node.private || ancestorPrivate" name="lock" :size="12" class="tree-lock" /></RouterLink>
      </div>
      <DocTree
        v-if="node.children?.length && openMap[node.id]"
        :nodes="node.children"
        :active-id="activeId"
        :depth="depth + 1"
        :ancestor-private="ancestorPrivate || node.private"
      />
    </li>
  </ul>
</template>
