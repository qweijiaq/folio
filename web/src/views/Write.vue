<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { buildTree, flattenTree } from '../utils/tree'
import CoverImage from '../components/CoverImage.vue'
import MarkdownView from '../components/MarkdownView.vue'
import Icon from '../components/Icon.vue'
import RevisionHistory from '../components/RevisionHistory.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

usePageMeta(() => ({ title: '写作', description: '知微编辑器', noindex: true }))

const form = reactive({
  kind: route.query.kind === 'doc' ? 'doc' : 'article',
  title: '',
  abstract: '',
  content: '',
  tags: [],
  status: 'draft',
  publishAt: '',
  parentID: route.query.parent ? Number(route.query.parent) : null,
  categoryID: null,
  seriesID: null,
  seriesOrder: 0,
  private: false,
  pinned: false,
  openComment: true,
  sort: 0
})

const tagInput = ref('')
const textarea = ref(null)
const fileInput = ref(null)
const doc = ref(null)
const parentItems = ref([])
const categories = ref([])
const seriesOptions = ref([])
const loading = ref(true)
const saving = ref('')
const preview = ref(true)
const uploading = ref(false)
const aiEnabled = ref(false)
const aiLoading = ref(false)
const savedSnapshot = ref('')
const draftAvailable = ref(null)
const revisionOpen = ref(false)
const draftSavedAt = ref(0)
let autoSaveTimer = null

function toLocalInput(value) {
  if (!value) return ''
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (n) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function scheduleText(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

const editingId = computed(() => (route.params.id ? Number(route.params.id) : 0))
const isEdit = computed(() => Boolean(editingId.value))
const wordCount = computed(() => (form.content || '').replace(/\s/g, '').length)
const coverSeed = computed(() => editingId.value || `new-${form.kind}`)
const coverCategory = computed(() => categories.value.find((item) => String(item.id) === String(form.categoryID))?.name || '')
const minPublishAt = computed(() => toLocalInput(new Date(Date.now() + 60 * 1000)))
const publishAction = computed(() => (form.publishAt && form.status !== 'published' ? 'scheduled' : 'published'))
const statusLabel = computed(() => {
  if (dirty.value) return '未保存'
  if (form.status === 'published') return '已发布'
  if (form.status === 'scheduled') return '定时发布'
  return '草稿'
})
const publishLabel = computed(() => {
  if (saving.value === 'scheduled') return '定时中…'
  if (saving.value === 'published') return '发布中…'
  if (form.publishAt && form.status !== 'published') return form.status === 'scheduled' ? '更新定时' : '定时发布'
  return form.status === 'published' ? '更新并发布' : '发布'
})
const parentPrivate = computed(() => {
  if (form.kind !== 'doc' || !form.parentID) return false
  const byId = new Map(parentItems.value.map((item) => [Number(item.id), item]))
  let current = byId.get(Number(form.parentID))
  for (let i = 0; i < 20 && current; i += 1) {
    if (current.private) return true
    current = current.parentID ? byId.get(Number(current.parentID)) : null
  }
  return false
})

function snapshot() {
  return JSON.stringify({
    kind: form.kind,
    title: form.title,
    abstract: form.abstract,
    content: form.content,
    tags: form.tags,
    status: form.status,
    publishAt: form.publishAt,
    parentID: form.parentID,
    categoryID: form.categoryID,
    seriesID: form.seriesID,
    seriesOrder: form.seriesOrder,
    private: form.private,
    pinned: form.pinned,
    openComment: form.openComment,
    sort: form.sort
  })
}

const dirty = computed(() => !loading.value && snapshot() !== savedSnapshot.value)
const draftTime = computed(() => {
  if (!draftAvailable.value?.savedAt) return ''
  return new Date(draftAvailable.value.savedAt).toLocaleString('zh-CN', {
    month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit'
  })
})
const autoSavedTime = computed(() => {
  if (!draftSavedAt.value) return ''
  return new Date(draftSavedAt.value).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
})

function draftKey() {
  const owner = auth.user?.id || 0
  return editingId.value ? `folio.draft:${owner}:${editingId.value}` : `folio.draft:${owner}:new:${form.kind}`
}

function draftFields(source) {
  return {
    kind: source.kind,
    title: source.title || '',
    abstract: source.abstract || '',
    content: source.content || '',
    tags: Array.isArray(source.tags) ? source.tags : [],
    status: source.status || 'draft',
    publishAt: source.publishAt ? toLocalInput(source.publishAt) : '',
    parentID: source.parentID ?? null,
    categoryID: source.categoryID ?? null,
    seriesID: source.seriesID ?? null,
    seriesOrder: Number(source.seriesOrder) || 0,
    private: Boolean(source.private),
    pinned: Boolean(source.pinned),
    openComment: source.openComment !== false,
    sort: Number(source.sort) || 0
  }
}

function loadLocalDraft() {
  try {
    const raw = localStorage.getItem(draftKey())
    if (!raw) return
    const data = JSON.parse(raw)
    if (!data || !data.savedAt) {
      localStorage.removeItem(draftKey())
      return
    }
    if (isEdit.value && doc.value) {
      const serverTime = new Date(doc.value.updatedAt).getTime()
      if (Number(data.savedAt) < serverTime) {
        localStorage.removeItem(draftKey())
        return
      }
    }
    if (JSON.stringify(draftFields(data)) === snapshot()) {
      localStorage.removeItem(draftKey())
      return
    }
    draftAvailable.value = { ...draftFields(data), savedAt: data.savedAt }
  } catch {
    localStorage.removeItem(draftKey())
  }
}

function restoreLocalDraft() {
  if (!draftAvailable.value) return
  Object.assign(form, draftFields(draftAvailable.value))
  draftAvailable.value = null
  toast('已恢复本地草稿，记得保存')
}

function discardLocalDraft() {
  draftAvailable.value = null
  try {
    localStorage.removeItem(draftKey())
  } catch {
    /* ignore */
  }
  toast('已丢弃本地草稿')
}

function autoSaveLocalDraft() {
  try {
    const key = draftKey()
    if (loading.value || saving.value || !dirty.value) {
      if (!loading.value && !draftAvailable.value) localStorage.removeItem(key)
      if (!draftAvailable.value) draftSavedAt.value = 0
      return
    }
    const savedAt = Date.now()
    localStorage.setItem(key, JSON.stringify({ ...draftFields(form), savedAt }))
    draftSavedAt.value = savedAt
  } catch {
    /* localStorage 不可用时静默降级 */
  }
}

const parentOptions = computed(() => {
  if (form.kind !== 'doc') return []
  const blocked = new Set()
  if (isEdit.value) {
    blocked.add(editingId.value)
    let changed = true
    while (changed) {
      changed = false
      parentItems.value.forEach((item) => {
        if (item.parentID && blocked.has(item.parentID) && !blocked.has(item.id)) {
          blocked.add(item.id)
          changed = true
        }
      })
    }
  }
  return parentItems.value.filter((item) => !blocked.has(item.id))
})

async function load() {
  loading.value = true
  try {
    const [treeData, mine, categoryData, seriesData] = await Promise.all([
      api.get('/doc-tree'),
      api.get('/me/docs', { kind: 'doc', pageSize: 50 }),
      api.get('/categories'),
      api.get('/me/series')
    ])
    categories.value = categoryData.items
    seriesOptions.value = seriesData.items || []
    api.get('/ai/status').then((data) => {
      aiEnabled.value = Boolean(data.enabled)
    }).catch(() => {
      aiEnabled.value = false
    })
    const map = new Map()
    ;[...treeData.items, ...mine.items].forEach((item) => map.set(item.id, item))
    parentItems.value = flattenTree(buildTree([...map.values()]))

    if (isEdit.value) {
      const data = await api.get(`/docs/${editingId.value}/edit`)
      doc.value = data.doc
      form.kind = data.doc.kind
      form.title = data.doc.title
      form.abstract = data.doc.abstract || ''
      form.content = data.doc.content || ''
      form.tags = [...(data.doc.tags || [])]
      form.status = data.doc.status
      form.publishAt = data.doc.status === 'scheduled' && data.doc.publishAt ? toLocalInput(data.doc.publishAt) : ''
      form.parentID = data.doc.parentID ?? null
      form.categoryID = data.doc.categoryID ?? null
      form.seriesID = data.doc.seriesID ?? null
      form.seriesOrder = data.doc.seriesOrder || 0
      form.private = Boolean(data.doc.private)
      form.pinned = data.doc.pinned
      form.openComment = data.doc.openComment !== false
      form.sort = data.doc.sort || 0
    }
  } catch (err) {
    toastError(err)
    if (err.status === 403 || err.status === 404) router.replace('/me')
  } finally {
    loading.value = false
    savedSnapshot.value = snapshot()
    loadLocalDraft()
  }
}

function addTag() {
  const value = tagInput.value.trim().replace(/^#/, '')
  if (!value) return
  if (form.tags.length >= 6) {
    toast('最多添加 6 个标签')
    return
  }
  if (!form.tags.some((tag) => tag.toLowerCase() === value.toLowerCase())) {
    form.tags.push(value.slice(0, 16))
  }
  tagInput.value = ''
}

function onTagKey(event) {
  if (event.key === ',' || event.key === '，') {
    event.preventDefault()
    addTag()
  }
  if (event.key === 'Backspace' && !tagInput.value && form.tags.length) {
    form.tags.pop()
  }
}

function removeTag(tag) {
  form.tags = form.tags.filter((item) => item !== tag)
}

function wrap(before, after = '', placeholder = '文字') {
  const el = textarea.value
  if (!el) return
  const start = el.selectionStart
  const end = el.selectionEnd
  const selected = form.content.slice(start, end) || placeholder
  form.content = form.content.slice(0, start) + before + selected + after + form.content.slice(end)
  nextTick(() => {
    el.focus()
    el.setSelectionRange(start + before.length, start + before.length + selected.length)
  })
}

function prefix(marker) {
  const el = textarea.value
  if (!el) return
  const start = el.selectionStart
  const lineStart = form.content.lastIndexOf('\n', start - 1) + 1
  form.content = form.content.slice(0, lineStart) + marker + form.content.slice(lineStart)
  nextTick(() => {
    el.focus()
    el.setSelectionRange(start + marker.length, start + marker.length)
  })
}

function insertAtPosition(snippet, position = null) {
  const el = textarea.value
  if (!el) return
  const start = position === null ? el.selectionStart : position
  const end = position === null ? el.selectionEnd : position
  form.content = form.content.slice(0, start) + snippet + form.content.slice(end)
  nextTick(() => {
    el.focus()
    const pos = start + snippet.length
    el.setSelectionRange(pos, pos)
  })
}

function insertSnippet(snippet) {
  insertAtPosition(snippet)
}

function insertLink() {
  wrap('[', '](https://)', '链接文字')
}

async function uploadImages(fileList) {
  const images = Array.from(fileList || []).filter((file) => file.type.startsWith('image/'))
  if (!images.length || uploading.value) return
  const el = textarea.value
  const position = el ? el.selectionStart : form.content.length
  uploading.value = true
  try {
    const uploaded = []
    for (const file of images) {
      uploaded.push(await api.uploadImage(file))
    }
    const markdown = uploaded
      .map((image) => `![${(image.filename || '图片').replace(/[[\]]/g, '')}](${image.url})`)
      .join('\n\n')
    const before = position > 0 && form.content[position - 1] !== '\n' ? '\n\n' : ''
    const after = form.content[position] && form.content[position] !== '\n' ? '\n\n' : ''
    insertAtPosition(before + markdown + after, position)
    toast(`已插入 ${uploaded.length} 张图片`)
  } catch (err) {
    toastError(err)
  } finally {
    uploading.value = false
  }
}

function onFilePick(event) {
  const files = Array.from(event.target.files || [])
  event.target.value = ''
  uploadImages(files)
}

function onPaste(event) {
  const files = Array.from(event.clipboardData?.files || []).filter((file) => file.type.startsWith('image/'))
  if (!files.length) return
  event.preventDefault()
  uploadImages(files)
}

function onDrop(event) {
  const files = Array.from(event.dataTransfer?.files || []).filter((file) => file.type.startsWith('image/'))
  if (!files.length) return
  event.preventDefault()
  uploadImages(files)
}

function onDragOver(event) {
  if (Array.from(event.dataTransfer?.types || []).includes('Files')) {
    event.preventDefault()
  }
}

async function aiAnalyze() {
  if (!form.content.trim()) {
    toast('先写一点正文，再让 AI 提炼')
    return
  }
  aiLoading.value = true
  try {
    const data = await api.post('/ai/analyze', { content: form.content, title: form.title })
    const changed = []
    if (!form.title.trim() && data.title) {
      form.title = data.title
      changed.push('标题')
    }
    if (!form.abstract.trim() && data.abstract) {
      form.abstract = data.abstract
      changed.push('摘要')
    }
    if (Array.isArray(data.tags) && data.tags.length) {
      let added = 0
      for (const tag of data.tags) {
        if (form.tags.length >= 6) break
        const value = String(tag).trim()
        if (!value) continue
        if (form.tags.some((item) => item.toLowerCase() === value.toLowerCase())) continue
        form.tags.push(value)
        added += 1
      }
      if (added) changed.push(`${added} 个标签`)
    }
    toast(changed.length ? `AI 已生成${changed.join('、')}` : 'AI 没有需要补充的内容')
  } catch (err) {
    toastError(err)
  } finally {
    aiLoading.value = false
  }
}

async function save(status = 'draft') {
  if (saving.value) return
  const title = form.title.trim()
  if (!title) {
    toast('请先填写标题')
    return
  }
  if ((status === 'published' || status === 'scheduled') && !form.content.trim()) {
    toast('发布前请先写正文')
    return
  }
  if (status === 'scheduled') {
    if (!form.publishAt) {
      toast('请选择定时发布时间')
      return
    }
    const when = new Date(form.publishAt)
    if (Number.isNaN(when.getTime())) {
      toast('定时发布时间格式不正确')
      return
    }
    if (form.status !== 'scheduled' && when.getTime() <= Date.now()) {
      toast('定时发布时间需晚于现在')
      return
    }
  }
  addTag()
  saving.value = status
  const payload = {
    kind: form.kind,
    title,
    abstract: form.abstract.trim(),
    content: form.content,
    tags: form.tags,
    status,
    publishAt: status === 'scheduled' && form.publishAt ? new Date(form.publishAt).toISOString() : null,
    parentID: form.kind === 'doc' && form.parentID ? Number(form.parentID) : null,
    categoryID: form.kind === 'article' && form.categoryID ? Number(form.categoryID) : null,
    seriesID: form.kind === 'article' && form.seriesID ? Number(form.seriesID) : null,
    seriesOrder: form.kind === 'article' ? Number(form.seriesOrder) || 0 : 0,
    private: form.kind === 'doc' ? form.private : false,
    pinned: form.pinned,
    openComment: form.openComment,
    sort: Number(form.sort) || 0
  }
  try {
    if (isEdit.value) {
      const data = await api.put(`/docs/${editingId.value}`, payload)
      doc.value = data.doc
      form.status = data.doc.status || status
      form.publishAt = data.doc.status === 'scheduled' && data.doc.publishAt ? toLocalInput(data.doc.publishAt) : ''
      savedSnapshot.value = snapshot()
      try {
        localStorage.removeItem(draftKey())
      } catch {
        /* ignore */
      }
      draftSavedAt.value = 0
      toast(form.status === 'scheduled' ? '已更新定时发布' : form.status === 'published' ? '已更新并保持发布' : '已转为草稿')
    } else {
      const data = await api.post('/docs', payload)
      form.status = data.doc.status || status
      form.publishAt = data.doc.status === 'scheduled' && data.doc.publishAt ? toLocalInput(data.doc.publishAt) : ''
      savedSnapshot.value = snapshot()
      try {
        localStorage.removeItem(draftKey())
      } catch {
        /* ignore */
      }
      draftSavedAt.value = 0
      toast(form.status === 'scheduled' ? `已安排在 ${scheduleText(form.publishAt)} 发布` : form.status === 'published' ? '已发布' : '草稿已保存')
      if (form.status === 'published') {
        router.push(`/${form.kind === 'doc' ? 'docs' : 'articles'}/${data.doc.id}`)
      } else {
        router.push({ path: '/me', query: { tab: form.kind === 'doc' ? 'docs' : 'articles' } })
      }
    }
  } catch (err) {
    toastError(err)
  } finally {
    saving.value = ''
  }
}

function onRevisionRestored(restored) {
  doc.value = restored
  form.kind = restored.kind
  form.title = restored.title
  form.abstract = restored.abstract || ''
  form.content = restored.content || ''
  form.tags = [...(restored.tags || [])]
  form.status = restored.status
  form.publishAt = restored.status === 'scheduled' && restored.publishAt ? toLocalInput(restored.publishAt) : ''
  form.parentID = restored.parentID ?? null
  form.categoryID = restored.categoryID ?? null
  form.seriesID = restored.seriesID ?? null
  form.seriesOrder = restored.seriesOrder || 0
  form.private = Boolean(restored.private)
  form.pinned = restored.pinned
  form.openComment = restored.openComment !== false
  form.sort = restored.sort || 0
  savedSnapshot.value = snapshot()
  draftAvailable.value = null
  try {
    localStorage.removeItem(draftKey())
  } catch {
    /* ignore */
  }
  revisionOpen.value = false
  toast('已恢复历史版本')
}

function goBack() {
  if (isEdit.value) router.push(`/${form.kind === 'doc' ? 'docs' : 'articles'}/${editingId.value}`)
  else router.back()
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm('这篇内容还有未保存的修改，确定离开吗？')
})

function onBeforeUnload(event) {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

function onKeydown(event) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    const next = form.status === 'scheduled' && form.publishAt ? 'scheduled' : form.status === 'published' ? 'published' : 'draft'
    save(next)
  }
}

watch(editingId, load)

onMounted(() => {
  load()
  autoSaveTimer = window.setInterval(autoSaveLocalDraft, 3000)
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('beforeunload', onBeforeUnload)
})

onBeforeUnmount(() => {
  if (autoSaveTimer) window.clearInterval(autoSaveTimer)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('beforeunload', onBeforeUnload)
})
</script>

<template>
  <div class="page editor-page">
    <div class="container">
      <header class="editor-head">
        <div class="editor-head-left">
          <button class="icon-btn" type="button" aria-label="返回" @click="goBack">
            <Icon name="arrowLeft" :size="18" />
          </button>
          <div>
            <span class="eyebrow">{{ isEdit ? '编辑' : '创作' }}</span>
            <h1>{{ form.title || '未命名' }}</h1>
          </div>
        </div>
        <div class="editor-actions">
          <span class="editor-status" :class="[form.status, { dirty }]">{{ statusLabel }}</span>
          <button v-if="isEdit" class="btn btn-ghost btn-sm" type="button" @click="revisionOpen = true">历史版本</button>
          <button class="btn btn-ghost btn-sm" type="button" :disabled="Boolean(saving)" @click="save('draft')">
            {{ saving === 'draft' ? '保存中…' : '存为草稿' }}
          </button>
          <button class="btn btn-primary btn-sm" type="button" :disabled="Boolean(saving)" @click="save(publishAction)">
            {{ publishLabel }}
          </button>
        </div>
      </header>

      <div v-if="draftAvailable && !loading" class="draft-recover">
        <span><Icon name="clock" :size="15" /> 发现 {{ draftTime }} 自动保存的本地草稿</span>
        <button class="btn btn-primary btn-sm" type="button" @click="restoreLocalDraft">恢复草稿</button>
        <button class="btn btn-ghost btn-sm" type="button" @click="discardLocalDraft">丢弃</button>
      </div>

      <p v-if="loading" class="muted loading-line">正在准备编辑器…</p>

      <div v-else class="editor-shell">
        <aside class="editor-fields">
          <div class="field">
            <label for="editor-title">标题</label>
            <input id="editor-title" v-model="form.title" class="title-input" maxlength="120" placeholder="给这篇内容一个名字" />
          </div>

          <div class="field">
            <label for="editor-abstract">摘要 <span class="hint">留空则自动截取正文</span></label>
            <textarea id="editor-abstract" v-model="form.abstract" rows="3" maxlength="500" placeholder="一句话说明它讲了什么…"></textarea>
          </div>

          <div class="field">
            <label>类型</label>
            <div class="segmented" :class="{ disabled: isEdit }">
              <button type="button" :class="{ active: form.kind === 'article' }" @click="!isEdit && (form.kind = 'article')">文章</button>
              <button type="button" :class="{ active: form.kind === 'doc' }" @click="!isEdit && (form.kind = 'doc')">文档</button>
            </div>
            <p class="hint">{{ form.kind === 'doc' ? '文档可以设置上级，组成左侧目录树。' : '文章按时间排列，出现在文章列表。' }}</p>
          </div>

          <div v-if="form.kind === 'article'" class="field">
            <label for="editor-category">分类 <span class="hint">可选</span></label>
            <select id="editor-category" v-model="form.categoryID">
              <option :value="null">未分类</option>
              <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option>
            </select>
          </div>

          <div v-if="form.kind === 'article'" class="form-row">
            <div class="field">
              <label for="editor-series">所属合集 <span class="hint">可选</span></label>
              <select id="editor-series" v-model="form.seriesID">
                <option :value="null">不属于合集</option>
                <option v-for="series in seriesOptions" :key="series.id" :value="series.id">{{ series.title }}（{{ series.count }}）</option>
              </select>
            </div>
            <div class="field">
              <label for="editor-series-order">合集内排序</label>
              <input id="editor-series-order" v-model.number="form.seriesOrder" type="number" min="0" :disabled="!form.seriesID" />
            </div>
          </div>

          <div class="field">
            <label>标签 <span class="hint">最多 6 个</span></label>
            <div class="tag-editor">
              <span v-for="tag in form.tags" :key="tag" class="tag">
                {{ tag }}
                <button type="button" aria-label="移除标签" @click="removeTag(tag)">
                  <Icon name="close" :size="12" />
                </button>
              </span>
              <input
                v-model="tagInput"
                maxlength="16"
                placeholder="回车添加"
                @keydown.enter.prevent="addTag"
                @keydown="onTagKey"
                @blur="addTag"
              />
            </div>
          </div>

          <div class="field">
            <label for="editor-publish-at">定时发布 <span class="hint">可选</span></label>
            <input id="editor-publish-at" v-model="form.publishAt" type="datetime-local" :min="minPublishAt" />
            <p v-if="form.publishAt && form.status !== 'published'" class="hint">
              点击「{{ form.status === 'scheduled' ? '更新定时' : '定时发布' }}」后，内容会在 {{ scheduleText(form.publishAt) }} 自动公开。
            </p>
            <p v-else class="hint">留空则点击发布时立即公开。</p>
          </div>

          <template v-if="form.kind === 'doc'">
            <div class="field">
              <label for="editor-parent">上级文档</label>
              <select id="editor-parent" v-model="form.parentID">
                <option :value="null">无（顶级文档）</option>
                <option v-for="item in parentOptions" :key="item.id" :value="item.id">
                  {{ '　'.repeat(item.depth) }}{{ item.title }}{{ item.private ? '（私密）' : '' }}
                </option>
              </select>
            </div>
            <div class="form-row">
              <div class="field">
                <label for="editor-sort">同级排序</label>
                <input id="editor-sort" v-model.number="form.sort" type="number" min="0" />
              </div>
              <label class="checkbox">
                <input v-model="form.pinned" type="checkbox" />
                置顶
              </label>
            </div>
            <label class="checkbox">
              <input v-model="form.private" type="checkbox" />
              私密文档（只有自己可见）
            </label>
            <p v-if="form.private" class="hint">保存后只有你（和超级管理员）能在列表、目录和详情中看到。</p>
            <p v-else-if="parentPrivate" class="hint">上级文档是私密文档，这篇文档也会按私密处理。</p>
          </template>
          <label v-else class="checkbox">
            <input v-model="form.pinned" type="checkbox" />
            在列表中置顶
          </label>

          <label class="checkbox">
            <input v-model="form.openComment" type="checkbox" />
            允许读者评论
          </label>

          <div class="editor-note">
            <Icon name="info" :size="15" />
            <span>封面与头像由代码生成，不支持上传。正文图片可以直接粘贴、拖拽，或点工具栏的「图片」上传。</span>
          </div>
        </aside>

        <div class="editor-main">
          <div class="editor-cover">
            <div class="editor-cover-head">
              自动封面
              <span>由标题、摘要、标签与分类生成，随内容更新</span>
            </div>
            <CoverImage
              :seed="coverSeed"
              :title="form.title"
              :abstract="form.abstract"
              :tags="form.tags"
              :category="coverCategory"
              :kind="form.kind"
            />
          </div>

          <div class="editor-toolbar">
            <button type="button" title="二级标题" @click="prefix('## ')">H2</button>
            <button type="button" title="三级标题" @click="prefix('### ')">H3</button>
            <button type="button" title="加粗" @click="wrap('**', '**', '重点')"><strong>B</strong></button>
            <button type="button" title="斜体" @click="wrap('*', '*', '斜体')"><em>I</em></button>
            <button type="button" title="引用" @click="prefix('> ')">❝</button>
            <button type="button" title="行内代码" @click="wrap('`', '`', '代码')">&lt;/&gt;</button>
            <button type="button" title="链接" @click="insertLink">链接</button>
            <button type="button" title="上传图片（也支持粘贴、拖拽）" :disabled="uploading" @click="fileInput?.click()">
              {{ uploading ? '上传中…' : '图片' }}
            </button>
            <button v-if="aiEnabled" type="button" title="根据正文生成标题、摘要和标签" :disabled="aiLoading" @click="aiAnalyze">
              {{ aiLoading ? '思考中…' : 'AI 摘要' }}
            </button>
            <button type="button" title="无序列表" @click="prefix('- ')">列表</button>
            <button type="button" title="代码块" @click="insertSnippet('\n```go\n\n```\n')">代码块</button>
            <button type="button" title="分割线" @click="insertSnippet('\n\n---\n\n')">分割线</button>
            <span class="toolbar-spacer"></span>
            <button type="button" class="preview-toggle" @click="preview = !preview">{{ preview ? '隐藏预览' : '显示预览' }}</button>
          </div>

          <div class="editor-split" :class="{ 'no-preview': !preview }">
            <textarea
              ref="textarea"
              v-model="form.content"
              class="editor-input"
              placeholder="用 Markdown 开始写作。&#10;&#10;## 一个小标题&#10;&#10;先写下第一句，再慢慢修改。图片可以直接粘贴或拖进来。"
              @paste="onPaste"
              @drop="onDrop"
              @dragover="onDragOver"
            ></textarea>
            <div v-if="preview" class="editor-preview">
              <MarkdownView :source="form.content" />
            </div>
          </div>

          <input
            ref="fileInput"
            class="hidden-file"
            type="file"
            accept="image/jpeg,image/png,image/gif,image/webp"
            multiple
            @change="onFilePick"
          />

          <div class="editor-foot">
            <span>{{ wordCount }} 字<template v-if="autoSavedTime"> · 本地草稿已自动保存 {{ autoSavedTime }}</template></span>
            <span>支持 Markdown · 图片可粘贴/拖拽 · Ctrl/⌘ + S 保存草稿</span>
          </div>
        </div>
      </div>
      <RevisionHistory v-if="revisionOpen" :doc-id="editingId" @close="revisionOpen = false" @restored="onRevisionRestored" />
    </div>
  </div>
</template>
