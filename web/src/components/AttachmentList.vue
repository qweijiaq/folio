<script setup>
import Icon from './Icon.vue'
import { formatBytes } from '../utils/format'

defineProps({
  attachments: { type: Array, default: () => [] }
})

function sizeLabel(item) {
  if (item.kind === 'link') return '外部链接'
  return item.size ? formatBytes(item.size) : '附件文件'
}
</script>

<template>
  <section v-if="attachments?.length" class="attachment-section" aria-labelledby="attachment-title">
    <header class="attachment-head">
      <span class="eyebrow">附件</span>
      <h2 id="attachment-title">附件与资源</h2>
      <p>随正文一起保存的文件和外部链接。</p>
    </header>
    <div class="attachment-grid">
      <a
        v-for="(item, index) in attachments"
        :key="`${item.kind}-${item.url}-${index}`"
        class="attachment-card"
        :href="item.url"
        :download="item.kind === 'file' ? item.title : undefined"
        target="_blank"
        rel="noopener noreferrer"
      >
        <span class="attachment-icon">
          <Icon :name="item.kind === 'link' ? 'link' : 'cloud'" :size="18" />
        </span>
        <span class="attachment-main">
          <strong>{{ item.title }}</strong>
          <small>{{ sizeLabel(item) }}</small>
        </span>
        <span class="attachment-action" aria-hidden="true">
          <Icon :name="item.kind === 'link' ? 'arrowRight' : 'chevronDown'" :size="15" />
        </span>
      </a>
    </div>
  </section>
</template>
