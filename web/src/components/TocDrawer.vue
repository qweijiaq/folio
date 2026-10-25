<script setup>
import { nextTick, onBeforeUnmount, ref } from 'vue'
import Icon from './Icon.vue'

defineProps({
  items: { type: Array, default: () => [] },
  activeId: { type: String, default: '' },
  title: { type: String, default: '目录' }
})

const open = ref(false)
const panel = ref(null)
let previousOverflow = ''

function onKeydown(event) {
  if (event.key === 'Escape') close()
}

function openDrawer() {
  open.value = true
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  window.addEventListener('keydown', onKeydown)
  nextTick(() => panel.value?.focus())
}

function close() {
  if (!open.value) return
  open.value = false
  document.body.style.overflow = previousOverflow
  window.removeEventListener('keydown', onKeydown)
}

function jump(item) {
  close()
  const target = document.getElementById(item.id)
  if (target) {
    target.scrollIntoView({ behavior: 'smooth', block: 'start' })
    history.replaceState(null, '', `#${item.id}`)
  }
}

onBeforeUnmount(() => {
  if (open.value) {
    document.body.style.overflow = previousOverflow
    window.removeEventListener('keydown', onKeydown)
  }
})
</script>

<template>
  <button class="toc-fab" type="button" aria-label="打开目录" :aria-expanded="open" @click="openDrawer">
    <Icon name="list" :size="17" />
    目录
  </button>

  <Teleport to="body">
    <Transition name="toc-drawer">
      <div v-if="open" class="toc-drawer-overlay" @mousedown.self="close">
        <aside ref="panel" class="toc-drawer" role="dialog" aria-modal="true" :aria-label="title" tabindex="-1">
          <header class="toc-drawer-head">
            <span>{{ title }}</span>
            <button class="icon-btn" type="button" aria-label="关闭目录" @click="close">
              <Icon name="close" :size="18" />
            </button>
          </header>
          <nav class="toc">
            <a
              v-for="item in items"
              :key="item.id"
              :href="`#${item.id}`"
              :class="[`toc-level-${item.level}`, { active: activeId === item.id }]"
              :aria-current="activeId === item.id ? 'location' : undefined"
              @click.prevent="jump(item)"
            >
              {{ item.text }}
            </a>
          </nav>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>
