<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import JSZip from 'jszip'
import { api } from '../api/client'
import { toast, toastError } from '../stores/toast'
import MarkdownView from '../components/MarkdownView.vue'
import SelectBox from '../components/SelectBox.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

usePageMeta(() => ({ title: '发布助手', description: '导入 Markdown、上传图片、AI 审校并创建草稿', noindex: true }))

const router = useRouter()
const rawMarkdown = ref('')
const fileName = ref('')
const zipImages = ref([])
const chapters = ref([])
const bookTitle = ref('')
const uploading = ref(false)
const uploadedCount = ref(0)
const reviewing = ref(false)
const publishing = ref(false)
const issues = ref([])
const edits = ref([])
const revisedMarkdown = ref('')
const suggestedTitle = ref('')
const suggestedAbstract = ref('')
const suggestedTags = ref([])
const title = ref('')
const abstract = ref('')
const tagsText = ref('')
const targetKind = ref('doc')
const categoryID = ref(null)
const categories = ref([])
const parentID = ref(null)
const allDocs = ref([])
const autoTree = ref(true)
const rootTitle = ref('读书笔记')
const sectionTitle = ref('')
const status = ref('draft')
const searchHidden = ref(false)
const perspective = ref('god')
const removeAIFlavor = ref(true)
const batchPublish = ref(false)

const perspectiveOptions = [
  { value: 'god', label: '上帝视角（不用我/你）' },
  { value: 'first', label: '第一人称（我）' },
  { value: 'third', label: '第三人称（他/她）' }
]
const statusOptions = [
  { value: 'draft', label: '草稿（推荐先审后发）' },
  { value: 'published', label: '立即发布' }
]
const categoryOptions = computed(() => [
  { value: null, label: '未分类' },
  ...categories.value.map((item) => ({ value: item.id, label: item.name }))
])
const parentOptions = computed(() => {
  const byId = new Map(allDocs.value.map((item) => [item.id, item]))
  function depth(doc) {
    let value = 0
    let current = doc
    const seen = new Set()
    while (current?.parentID && value < 20 && !seen.has(current.id)) {
      seen.add(current.id)
      current = byId.get(current.parentID)
      value += 1
    }
    return value
  }
  return [
    { value: null, label: '无（顶级文档）' },
    ...allDocs.value.map((item) => ({
      value: item.id,
      label: `${'　'.repeat(depth(item))}${item.title}${item.status === 'draft' ? '（草稿）' : ''}`
    }))
  ]
})
const tags = computed(() => parseTags(tagsText.value))
const canPublish = computed(() => Boolean(title.value.trim() && rawMarkdown.value.trim()))

function parseTags(value) {
  const seen = new Set()
  const items = []
  for (const part of String(value || '').split(/[、,，;；\s]+/)) {
    const tag = part.trim().replace(/^#+/, '').slice(0, 16)
    if (!tag) continue
    const key = tag.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    items.push(tag)
    if (items.length >= 6) break
  }
  return items
}

function extractFrontmatter(text) {
  const match = String(text || '').match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/)
  if (!match) return { title: '', content: text }
  const titleMatch = match[1].match(/^title:\s*["']?(.*?)["']?\s*$/m)
  return {
    title: titleMatch?.[1]?.trim() || '',
    content: String(text).slice(match[0].length)
  }
}

function guessTitle(text, fallback) {
  const front = extractFrontmatter(text).title
  if (front) return front
  const heading = String(text || '').match(/^#\s+(.+)$/m)
  return heading?.[1]?.trim() || fallback || ''
}

function loadMarkdown(text, fallbackTitle) {
  const parsed = extractFrontmatter(text)
  rawMarkdown.value = parsed.content || text
  title.value = parsed.title || guessTitle(text, fallbackTitle) || title.value
  revisedMarkdown.value = ''
  issues.value = []
  edits.value = []
}

function replaceImagePaths(text, map) {
  let output = text
  for (const [name, url] of Object.entries(map)) {
    const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    output = output.replace(new RegExp(`(?:\\.\\./)*(?:images\\/)?${escaped}`, 'g'), url)
  }
  return output
}

async function onFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  fileName.value = file.name
  try {
    if (/\.md$/i.test(file.name)) {
      loadMarkdown(await file.text(), file.name.replace(/\.md$/i, ''))
      chapters.value = []
      zipImages.value = []
      toast('Markdown 已读取')
      return
    }
    if (!/\.zip$/i.test(file.name)) {
      toast('只支持 .md 或 .zip 文件')
      return
    }
    const zip = await JSZip.loadAsync(file)
    const markdownFiles = Object.values(zip.files).filter((item) => !item.dir && /\.md$/i.test(item.name))
    const imageFiles = Object.values(zip.files).filter((item) => !item.dir && /\.(png|jpe?g|gif|webp)$/i.test(item.name))
    zipImages.value = imageFiles.map((item) => ({ name: item.name.split('/').pop(), entry: item }))
    if (!markdownFiles.length) {
      toast('zip 里没有找到 Markdown 文件')
      return
    }
    const bookEntry = markdownFiles.find((item) => /(^|\/)book\.md$/i.test(item.name))
    const chapterEntries = markdownFiles
      .filter((item) => item !== bookEntry)
      .sort((a, b) => a.name.localeCompare(b.name))
    bookTitle.value = bookEntry ? guessTitle(await bookEntry.async('string'), file.name.replace(/\.zip$/i, '')) : ''

    if (bookEntry) {
      const list = []
      for (const entry of chapterEntries) {
        const text = await entry.async('string')
        list.push({
          name: entry.name,
          title: guessTitle(text, entry.name.replace(/\.md$/i, '')),
          content: extractFrontmatter(text).content
        })
      }
      chapters.value = list
      if (list.length) {
        loadMarkdown(list[0].content, list[0].title)
      } else {
        loadMarkdown(extractFrontmatter(await bookEntry.async('string')).content, bookTitle.value)
      }
      if (!sectionTitle.value.trim()) sectionTitle.value = bookTitle.value
    } else {
      const first = chapterEntries[0] || markdownFiles[0]
      loadMarkdown(await first.async('string'), first.name.replace(/\.md$/i, ''))
      chapters.value = []
    }

    if (chapterEntries.length > 1) {
      const list = []
      for (const entry of chapterEntries) {
        const text = await entry.async('string')
        list.push({
          name: entry.name,
          title: guessTitle(text, entry.name.replace(/\.md$/i, '')),
          content: extractFrontmatter(text).content
        })
      }
      chapters.value = list
    }

    toast(`已读取 ${markdownFiles.length} 个 Markdown、${zipImages.value.length} 张图片`)
  } catch (err) {
    toastError(err)
  }
}

async function uploadZipImages() {
  if (!zipImages.value.length || uploading.value) return
  uploading.value = true
  uploadedCount.value = 0
  try {
    const map = {}
    for (const image of zipImages.value) {
      const blob = await image.entry.async('blob')
      const ext = image.name.split('.').pop().toLowerCase()
      const type = ext === 'jpg' || ext === 'jpeg' ? 'image/jpeg' : `image/${ext}`
      const file = new File([blob], image.name, { type })
      const uploaded = await api.uploadImage(file)
      map[image.name] = uploaded.url
      uploadedCount.value += 1
    }
    rawMarkdown.value = replaceImagePaths(rawMarkdown.value, map)
    chapters.value = chapters.value.map((chapter) => ({
      ...chapter,
      content: replaceImagePaths(chapter.content, map)
    }))
    toast(`已上传 ${uploadedCount.value} 张图片并替换路径`)
  } catch (err) {
    toastError(err)
  } finally {
    uploading.value = false
  }
}

function applyEdits(text, list) {
  let output = text
  for (const edit of list) {
    if (!edit.find || !output.includes(edit.find)) continue
    output = output.replace(edit.find, edit.replace)
  }
  return output
}

async function review() {
  if (!rawMarkdown.value.trim()) {
    toast('先导入或粘贴正文')
    return
  }
  reviewing.value = true
  try {
    const data = await api.post('/admin/publish/review', {
      content: rawMarkdown.value,
      title: title.value,
      abstract: abstract.value,
      tags: tags.value,
      perspective: perspective.value,
      mode: removeAIFlavor.value ? 'full' : 'proofread',
      kind: targetKind.value
    })
    issues.value = data.issues || []
    edits.value = data.edits || []
    suggestedTitle.value = data.title || ''
    suggestedAbstract.value = data.abstract || ''
    suggestedTags.value = data.tags || []
    revisedMarkdown.value = applyEdits(rawMarkdown.value, edits.value)
    toast(`审校完成：${issues.value.length} 个问题，${edits.value.length} 条修改建议`)
  } catch (err) {
    toastError(err)
  } finally {
    reviewing.value = false
  }
}

function applyAllEdits() {
  rawMarkdown.value = revisedMarkdown.value || applyEdits(rawMarkdown.value, edits.value)
  toast('已应用修改，可以继续预览或再次审校')
}

function useSuggestions() {
  if (suggestedTitle.value && !title.value.trim()) title.value = suggestedTitle.value
  if (suggestedAbstract.value && !abstract.value.trim()) abstract.value = suggestedAbstract.value
  if (suggestedTags.value.length) {
    tagsText.value = [...new Set([...tags.value, ...suggestedTags.value])].slice(0, 6).join('、')
  }
  toast('已填入可用的标题、摘要和标签')
}

async function ensureDoc(docTitle, parent) {
  const clean = docTitle.trim()
  if (!clean) return null
  const existing = allDocs.value.find(
    (item) => item.title === clean && (parent ? item.parentID === parent : !item.parentID)
  )
  if (existing) return existing.id
  const data = await api.post('/docs', {
    kind: 'doc',
    title: clean,
    abstract: `${clean}目录。`,
    content: `# ${clean}\n\n这里收录相关文档。`,
    status: 'published',
    parentID: parent,
    searchHidden: true
  })
  allDocs.value.push(data.doc)
  return data.doc.id
}

async function publish() {
  if (!canPublish.value) {
    toast('请填写标题和正文')
    return
  }
  publishing.value = true
  try {
    const base = {
      title: title.value.trim(),
      abstract: abstract.value.trim(),
      content: rawMarkdown.value,
      tags: tags.value,
      status: status.value,
      openComment: true
    }

    if (batchPublish.value && chapters.value.length > 1) {
      const rootId = autoTree.value ? await ensureDoc(rootTitle.value || '未命名目录', null) : null
      const sectionId = autoTree.value && sectionTitle.value.trim()
        ? await ensureDoc(sectionTitle.value, rootId)
        : rootId
      let created = 0
      for (const chapter of chapters.value) {
        await api.post('/docs', {
          ...base,
          kind: targetKind.value,
          title: chapter.title || '未命名章节',
          content: chapter.content,
          parentID: targetKind.value === 'doc' ? sectionId : null,
          categoryID: targetKind.value === 'article' ? categoryID.value : null,
          searchHidden: targetKind.value === 'doc' ? searchHidden.value : false
        })
        created += 1
      }
      toast(`已创建 ${created} 篇内容`)
      router.push(targetKind.value === 'doc' ? '/me?tab=docs' : '/me?tab=articles')
      return
    }

    let parent = parentID.value
    if (targetKind.value === 'doc' && autoTree.value) {
      const rootId = await ensureDoc(rootTitle.value || '未命名目录', null)
      parent = sectionTitle.value.trim()
        ? await ensureDoc(sectionTitle.value, rootId)
        : rootId
    }
    const data = await api.post('/docs', {
      ...base,
      kind: targetKind.value,
      parentID: targetKind.value === 'doc' ? parent : null,
      categoryID: targetKind.value === 'article' ? categoryID.value : null,
      searchHidden: targetKind.value === 'doc' ? searchHidden.value : false,
      seriesID: null,
      seriesOrder: 0,
      pinned: false,
      sort: 0
    })
    toast(status.value === 'published' ? '已发布' : '草稿已创建')
    const section = targetKind.value === 'doc' ? 'docs' : 'articles'
    router.push(status.value === 'published' ? `/${section}/${data.doc.id}` : `/write/${data.doc.id}`)
  } catch (err) {
    toastError(err)
  } finally {
    publishing.value = false
  }
}

onMounted(async () => {
  try {
    const [categoryData, treeData, mine] = await Promise.all([
      api.get('/categories'),
      api.get('/doc-tree'),
      api.get('/me/docs', { kind: 'doc', pageSize: 50 })
    ])
    categories.value = categoryData.items || []
    const map = new Map()
    ;[...(treeData.items || []), ...(mine.items || [])].forEach((item) => map.set(item.id, item))
    allDocs.value = [...map.values()]
  } catch (err) {
    toastError(err)
  }
})
</script>

<template>
  <div class="page publish-page">
    <div class="container">
      <header class="publish-head">
        <div>
          <span class="eyebrow">创作</span>
          <h1>发布助手</h1>
          <p class="page-lede">粘贴 Markdown 或上传 zip，自动上传图片、AI 审校、生成上级文档。</p>
        </div>
        <div class="publish-actions">
          <button class="btn btn-ghost btn-sm" type="button" :disabled="reviewing" @click="review">
            {{ reviewing ? '审校中…' : 'AI 审校' }}
          </button>
          <button class="btn btn-primary btn-sm" type="button" :disabled="publishing || !canPublish" @click="publish">
            {{ publishing ? '创建中…' : '创建草稿 / 发布' }}
          </button>
        </div>
      </header>

      <div class="publish-grid">
        <section class="publish-panel">
          <h2>1. 导入</h2>
          <div class="field">
            <label for="publish-file">Markdown 或 zip</label>
            <input id="publish-file" type="file" accept=".md,.zip" @change="onFile" />
            <p v-if="fileName" class="hint">已读取：{{ fileName }}</p>
            <p class="hint">zip 支持 book.md + chapters/*.md + images/*，会自动读取章节和图片。</p>
          </div>
          <div class="field">
            <label for="publish-content">正文 Markdown</label>
            <textarea id="publish-content" v-model="rawMarkdown" rows="20" placeholder="粘贴 Markdown，或上传 .md / .zip"></textarea>
          </div>
          <div v-if="zipImages.length" class="publish-images">
            <p>检测到 {{ zipImages.length }} 张图片</p>
            <button class="btn btn-ghost btn-sm" type="button" :disabled="uploading" @click="uploadZipImages">
              {{ uploading ? `上传中 ${uploadedCount}/${zipImages.length}…` : '上传图片并替换路径' }}
            </button>
          </div>
          <div v-if="chapters.length > 1" class="publish-images">
            <p>检测到 {{ chapters.length }} 个章节</p>
            <label class="checkbox">
              <input v-model="batchPublish" type="checkbox" />
              批量创建全部章节
            </label>
          </div>
        </section>

        <section class="publish-panel">
          <h2>2. 发布设置</h2>
          <div class="field">
            <label>类型</label>
            <div class="segmented">
              <button type="button" :class="{ active: targetKind === 'doc' }" @click="targetKind = 'doc'">文档</button>
              <button type="button" :class="{ active: targetKind === 'article' }" @click="targetKind = 'article'">文章</button>
            </div>
          </div>
          <div class="field">
            <label for="publish-title">标题</label>
            <input id="publish-title" v-model="title" maxlength="120" placeholder="标题" />
          </div>
          <div class="field">
            <label for="publish-abstract">摘要 <span class="hint">留空自动截取</span></label>
            <textarea id="publish-abstract" v-model="abstract" rows="3" maxlength="500" placeholder="一句话说明它讲了什么…"></textarea>
          </div>
          <div class="field">
            <label for="publish-tags">标签 <span class="hint">用、或逗号分隔</span></label>
            <input id="publish-tags" v-model="tagsText" placeholder="读书笔记、认知觉醒、成长" />
          </div>

          <div v-if="targetKind === 'article'" class="field">
            <label>分类</label>
            <SelectBox v-model="categoryID" :options="categoryOptions" placeholder="未分类" />
          </div>

          <template v-else>
            <label class="checkbox">
              <input v-model="autoTree" type="checkbox" />
              自动创建上级文档
            </label>
            <template v-if="autoTree">
              <div class="field">
                <label for="publish-root">顶级文档</label>
                <input id="publish-root" v-model="rootTitle" placeholder="读书笔记" />
              </div>
              <div class="field">
                <label for="publish-section">上级文档</label>
                <input id="publish-section" v-model="sectionTitle" :placeholder="bookTitle || '认知觉醒'" />
              </div>
              <p class="hint">自动创建的父级文档会发布为公开、搜索隐藏的目录页，方便直接形成树结构。</p>
            </template>
            <div v-else class="field">
              <label>上级文档</label>
              <SelectBox v-model="parentID" :options="parentOptions" placeholder="无（顶级文档）" />
            </div>
            <label class="checkbox">
              <input v-model="searchHidden" type="checkbox" />
              在搜索中隐藏（仅作为目录节点）
            </label>
          </template>

          <div class="field">
            <label>状态</label>
            <SelectBox v-model="status" :options="statusOptions" />
          </div>
          <div class="field">
            <label>AI 视角</label>
            <SelectBox v-model="perspective" :options="perspectiveOptions" />
          </div>
          <label class="checkbox">
            <input v-model="removeAIFlavor" type="checkbox" />
            去 AI 味
          </label>
          <p class="hint">审校会检查错别字、标点、标题层级、图注编号，并给出可应用的修改。</p>
        </section>
      </div>

      <section v-if="issues.length || edits.length" class="publish-panel">
        <h2>3. AI 审校结果</h2>
        <div v-if="issues.length">
          <h3>发现的问题</h3>
          <ul class="publish-issues">
            <li v-for="(issue, index) in issues" :key="index">{{ issue }}</li>
          </ul>
        </div>
        <div v-if="edits.length">
          <h3>修改建议</h3>
          <div v-for="(edit, index) in edits" :key="index" class="publish-edit">
            <p><strong>{{ edit.reason || '建议修改' }}</strong></p>
            <p class="publish-edit-find">原文：{{ edit.find }}</p>
            <p class="publish-edit-replace">改为：{{ edit.replace }}</p>
          </div>
          <button class="btn btn-primary btn-sm" type="button" @click="applyAllEdits">应用全部修改</button>
        </div>
        <div v-if="suggestedTitle || suggestedAbstract || suggestedTags.length" class="publish-suggest">
          <h3>标题 / 摘要 / 标签建议</h3>
          <p v-if="suggestedTitle">标题：{{ suggestedTitle }}</p>
          <p v-if="suggestedAbstract">摘要：{{ suggestedAbstract }}</p>
          <p v-if="suggestedTags.length">标签：{{ suggestedTags.join('、') }}</p>
          <button class="btn btn-ghost btn-sm" type="button" @click="useSuggestions">填入空字段</button>
        </div>
      </section>

      <section class="publish-panel">
        <h2>4. 预览</h2>
        <MarkdownView :source="rawMarkdown" />
      </section>
    </div>
  </div>
</template>
