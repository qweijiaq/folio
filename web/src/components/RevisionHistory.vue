<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api/client'
import { toast, toastError } from '../stores/toast'
import MarkdownView from './MarkdownView.vue'
import Icon from './Icon.vue'
import { buildLineDiff } from '../utils/diff'

const props = defineProps({
  docId: { type: [Number, String], required: true }
})
const emit = defineEmits(['close', 'restored'])

const revisions = ref([])
const selected = ref(null)
const currentDoc = ref(null)
const view = ref('diff')
const loading = ref(true)
const loadingDetail = ref(false)
const restoring = ref(false)
const diff = computed(() => buildLineDiff(selected.value?.content || '', currentDoc.value?.content || ''))

function formatTime(value) {
  if (!value) return ''
  return new Date(value).toLocaleString('zh-CN', {
    month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit'
  })
}

async function load() {
  loading.value = true
  try {
    const [data, edit] = await Promise.all([
      api.get(`/docs/${props.docId}/revisions`),
      api.get(`/docs/${props.docId}/edit`)
    ])
    revisions.value = data.items || []
    currentDoc.value = edit.doc || null
    if (revisions.value.length) await selectRevision(revisions.value[0])
  } catch (err) {
    toastError(err)
  } finally {
    loading.value = false
  }
}

async function selectRevision(item) {
  loadingDetail.value = true
  try {
    const data = await api.get(`/docs/${props.docId}/revisions/${item.id}`)
    selected.value = data.revision
  } catch (err) {
    toastError(err)
  } finally {
    loadingDetail.value = false
  }
}

async function restore() {
  if (!selected.value || restoring.value) return
  if (!window.confirm('恢复到这一版？当前内容会先保存为一条历史版本。')) return
  restoring.value = true
  try {
    const data = await api.post(`/docs/${props.docId}/revisions/${selected.value.id}/restore`)
    toast('已恢复历史版本')
    emit('restored', data.doc)
    emit('close')
  } catch (err) {
    toastError(err)
  } finally {
    restoring.value = false
  }
}

function onKeydown(event) {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => {
  document.body.style.overflow = 'hidden'
  window.addEventListener('keydown', onKeydown)
  load()
})

onBeforeUnmount(() => {
  document.body.style.overflow = ''
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="share-card" appear>
      <div class="revision-overlay" @mousedown.self="emit('close')">
        <section class="revision-panel" role="dialog" aria-modal="true" aria-label="历史版本">
          <header class="revision-head">
            <div>
              <span class="eyebrow">历史</span>
              <h2>版本记录</h2>
            </div>
            <button class="icon-btn" type="button" aria-label="关闭历史版本" @click="emit('close')">
              <Icon name="close" :size="18" />
            </button>
          </header>

          <p v-if="loading" class="muted loading-line">正在读取历史版本…</p>
          <div v-else-if="!revisions.length" class="empty-inline">还没有历史版本。每次编辑并保存后，旧内容会出现在这里。</div>
          <div v-else class="revision-body">
            <aside class="revision-list">
              <button
                v-for="item in revisions"
                :key="item.id"
                type="button"
                class="revision-item"
                :class="{ active: selected?.id === item.id }"
                @click="selectRevision(item)"
              >
                <strong>{{ item.title }}</strong>
                <span>{{ formatTime(item.createdAt) }} · {{ item.editorName || '未知' }}</span>
                <small>{{ item.excerpt }}</small>
              </button>
            </aside>

            <article class="revision-preview">
              <template v-if="loadingDetail">
                <p class="muted loading-line">正在读取这一版…</p>
              </template>
              <template v-else-if="selected">
                <header class="revision-preview-head">
                  <div>
                    <span class="badge">{{ selected.status === 'published' ? '已发布' : '草稿' }}</span>
                    <h3>{{ selected.title }}</h3>
                    <p>{{ formatTime(selected.createdAt) }} · {{ selected.editorName || '未知' }}</p>
                  </div>
                  <div class="revision-preview-actions">
                    <div class="revision-view-tabs" role="tablist" aria-label="版本查看方式">
                      <button
                        type="button"
                        role="tab"
                        :aria-selected="view === 'diff'"
                        :class="{ active: view === 'diff' }"
                        :disabled="!currentDoc"
                        @click="view = 'diff'"
                      >
                        与当前差异
                        <span class="revision-diff-stats">+{{ diff.stats.added }} −{{ diff.stats.removed }}</span>
                      </button>
                      <button type="button" role="tab" :aria-selected="view === 'preview'" :class="{ active: view === 'preview' }" @click="view = 'preview'">
                        预览
                      </button>
                    </div>
                    <button class="btn btn-primary btn-sm" type="button" :disabled="restoring" @click="restore">
                      {{ restoring ? '恢复中…' : '恢复这一版' }}
                    </button>
                  </div>
                </header>
                <div v-if="view === 'diff' && currentDoc" class="revision-diff" role="region" aria-label="与当前内容的差异">
                  <p v-if="!diff.rows.length" class="empty-inline">这一版与当前内容没有差异。</p>
                  <div
                    v-for="(row, index) in diff.rows"
                    :key="index"
                    class="revision-diff-row"
                    :class="row.type"
                  >
                    <span class="revision-diff-sign">{{ row.type === 'added' ? '+' : row.type === 'removed' ? '−' : ' ' }}</span>
                    <span class="revision-diff-text">{{ row.text || ' ' }}</span>
                  </div>
                </div>
                <div v-else class="revision-content">
                  <MarkdownView :source="selected.content" />
                </div>
              </template>
            </article>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
