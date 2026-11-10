<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import JSZip from 'jszip'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { confirmDialog } from '../stores/confirm'
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
const reusedCount = ref(0)
const imagesUploaded = ref(true)
const reviewing = ref(false)
const markdownFormatting = ref(false)
const markdownBackup = ref(null)
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
const referenceID = ref(null)
const referenceContent = ref('')
const referenceTitle = ref('')
const referenceLoading = ref(false)
const autoTree = ref(true)
const rootTitle = ref('读书笔记')
const sectionTitle = ref('')
const rootFocus = ref(false)
const sectionFocus = ref(false)
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
function docDepth(doc) {
  const byId = new Map(allDocs.value.map((item) => [item.id, item]))
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
const parentOptions = computed(() => [
  { value: null, label: '无（顶级文档）' },
  ...allDocs.value.map((item) => ({
    value: item.id,
    label: `${'　'.repeat(docDepth(item))}${item.title}${item.status === 'draft' ? '（草稿）' : ''}`
  }))
])
const referenceOptions = computed(() => [
  { value: null, label: '不参考' },
  ...allDocs.value.map((item) => ({
    value: item.id,
    label: `${'　'.repeat(docDepth(item))}${item.title}`
  }))
])

function sameTitle(a, b) {
  return String(a || '').trim().toLowerCase() === String(b || '').trim().toLowerCase()
}

const rootDocs = computed(() => allDocs.value.filter((item) => !item.parentID))
const rootMatch = computed(() => rootDocs.value.find((item) => sameTitle(item.title, rootTitle.value)))
const sectionPool = computed(() => {
  if (rootMatch.value) return allDocs.value.filter((item) => item.parentID === rootMatch.value.id)
  return allDocs.value.filter((item) => item.parentID)
})

function filterDocSuggestions(pool, query) {
  const value = String(query || '').trim().toLowerCase()
  const list = value ? pool.filter((item) => item.title.toLowerCase().includes(value)) : pool
  return list.slice(0, 6)
}

const rootSuggestions = computed(() => filterDocSuggestions(rootDocs.value, rootTitle.value))
const sectionSuggestions = computed(() => filterDocSuggestions(sectionPool.value, sectionTitle.value))
const rootHint = computed(() => {
  const value = rootTitle.value.trim()
  if (!value) return '留空会使用默认顶级文档。输入时会提示已有文档。'
  return rootMatch.value
    ? `将复用已有顶级文档「${rootMatch.value.title}」`
    : `将新建顶级文档「${value}」`
})
const sectionHint = computed(() => {
  const value = sectionTitle.value.trim()
  if (!value) return '留空会直接放在顶级文档下。'
  const parent = rootMatch.value
  const existing = parent && allDocs.value.find((item) => item.parentID === parent.id && sameTitle(item.title, value))
  return existing ? `将复用已有文档「${existing.title}」` : `将新建上级文档「${value}」`
})

function pickRoot(doc) {
  rootTitle.value = doc.title
  rootFocus.value = false
}

function pickSection(doc) {
  sectionTitle.value = doc.title
  if (doc.parentID) {
    const parent = allDocs.value.find((item) => item.id === doc.parentID)
    if (parent) rootTitle.value = parent.title
  }
  sectionFocus.value = false
}

function parentTitle(doc) {
  const parent = allDocs.value.find((item) => item.id === doc.parentID)
  return parent ? `属于 ${parent.title}` : '已有'
}

const tags = computed(() => parseTags(tagsText.value))
const isAbsoluteImage = (ref) => {
  const value = String(ref || '').trim().replace(/^<|>$/g, '')
  return !value || value.startsWith('#') || /^(https?:)?\/\//i.test(value) || value.startsWith('/') || value.startsWith('data:')
}
const hasRelativeImage = (markdown) => {
  const text = String(markdown || '')
  const inline = /!\[[^\]]*\]\(\s*(<[^>]+>|[^)\s]+)|<img\b[^>]*\bsrc=["']([^"']+)["']/gi
  let match
  while ((match = inline.exec(text))) {
    if (!isAbsoluteImage(match[1] || match[2] || '')) return true
  }
  const definitions = /^\s*\[[^\]]+\]:\s*(<[^>]+>|\S+)/gm
  while ((match = definitions.exec(text))) {
    if (!isAbsoluteImage(match[1])) return true
  }
  return false
}
const pendingImages = computed(() => {
  if (zipImages.value.length && !imagesUploaded.value) return true
  if (hasRelativeImage(rawMarkdown.value)) return true
  return chapters.value.some((chapter) => hasRelativeImage(chapter.content))
})
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

function normalizeImageRef(value) {
  let ref = String(value || '').trim().replace(/^<|>$/g, '')
  ref = ref.split(/[?#]/)[0].replace(/\\/g, '/')
  try {
    ref = decodeURIComponent(ref)
  } catch {
    /* keep original */
  }
  return ref.replace(/^(?:\.\.?\/)+/, '').replace(/^\/+/, '')
}

function replaceImagePaths(text, map) {
  const byPath = new Map()
  const byBase = new Map()
  for (const [name, url] of Object.entries(map)) {
    const norm = normalizeImageRef(name)
    if (!norm) continue
    byPath.set(norm, url)
    const base = norm.split('/').pop()
    if (base) {
      if (!byBase.has(base)) byBase.set(base, url)
      const lower = base.toLowerCase()
      if (!byBase.has(lower)) byBase.set(lower, url)
    }
  }
  const resolve = (raw) => {
    const norm = normalizeImageRef(raw)
    if (!norm) return null
    if (byPath.has(norm)) return byPath.get(norm)
    const base = norm.split('/').pop()
    return byBase.get(base) || byBase.get(base.toLowerCase()) || null
  }
  const replaceDest = (raw, prefix, suffix) => {
    const url = resolve(raw)
    return url ? `${prefix}${url}${suffix}` : `${prefix}${raw}${suffix}`
  }
  let output = String(text || '')
  // Markdown 图片 / 链接：![alt](images/a.png "title")
  output = output.replace(
    /(!?\[[^\]]*\]\(\s*)(<[^>]+>|[^)\s]+)((?:\s+["'][^"']*["'])?\s*\))/g,
    (_, prefix, dest, suffix) => replaceDest(dest, prefix, suffix)
  )
  // HTML <img src="...">
  output = output.replace(
    /(<img\b[^>]*?\bsrc=["'])([^"']+)(["'])/gi,
    (_, prefix, dest, suffix) => replaceDest(dest, prefix, suffix)
  )
  // 引用式图片：[id]: images/a.png
  output = output.replace(
    /^(\s*\[[^\]]+\]:\s*)(\S+)(.*)$/gm,
    (_, prefix, dest, suffix) => replaceDest(dest, prefix, suffix)
  )
  return output
}

async function handleFile(file) {
  if (!file) return
  fileName.value = file.name
  try {
    if (/\.md$/i.test(file.name)) {
      loadMarkdown(await file.text(), file.name.replace(/\.md$/i, ''))
      chapters.value = []
      zipImages.value = []
      imagesUploaded.value = true
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
    zipImages.value = imageFiles.map((item) => ({
      name: item.name.split('/').pop(),
      path: item.name,
      entry: item
    }))
    imagesUploaded.value = zipImages.value.length === 0
    uploadedCount.value = 0
    reusedCount.value = 0
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

    if (zipImages.value.length) {
      toast(`已读取 ${markdownFiles.length} 个 Markdown、${zipImages.value.length} 张图片，正在自动上传…`)
      await uploadZipImages()
    } else {
      toast(`已读取 ${markdownFiles.length} 个 Markdown`)
    }
  } catch (err) {
    toastError(err)
  }
}

async function onFile(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  await handleFile(file)
}

function onDrop(event) {
  const file = event.dataTransfer?.files?.[0]
  if (file) handleFile(file)
}

const IMAGE_CACHE_PREFIX = 'folio.publish.images.v1'

function imageCacheKey() {
  return `${IMAGE_CACHE_PREFIX}.${auth.user?.id || 'anon'}`
}

function loadImageCache() {
  try {
    const raw = localStorage.getItem(imageCacheKey())
    const parsed = raw ? JSON.parse(raw) : {}
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

function saveImageCache(cache) {
  try {
    const entries = Object.entries(cache)
    const trimmed = entries.length > 600 ? Object.fromEntries(entries.slice(-600)) : cache
    localStorage.setItem(imageCacheKey(), JSON.stringify(trimmed))
  } catch {
    /* 缓存失败不影响上传 */
  }
}

async function hashBlob(blob) {
  if (!globalThis.crypto?.subtle) return ''
  try {
    const digest = await crypto.subtle.digest('SHA-256', await blob.arrayBuffer())
    return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')
  } catch {
    return ''
  }
}

async function uploadZipImages() {
  if (!zipImages.value.length || uploading.value) return imagesUploaded.value
  uploading.value = true
  uploadedCount.value = 0
  reusedCount.value = 0
  imagesUploaded.value = false
  const failures = []
  const cache = loadImageCache()
  const seen = new Map()
  try {
    const map = {}
    for (const image of zipImages.value) {
      try {
        const blob = await image.entry.async('blob')
        const hash = await hashBlob(blob)
        let url = hash ? seen.get(hash) || cache[hash] : ''
        if (!url) {
          const ext = image.name.split('.').pop().toLowerCase()
          const type = ext === 'jpg' || ext === 'jpeg' ? 'image/jpeg' : `image/${ext}`
          const file = new File([blob], image.name, { type })
          const uploaded = await api.uploadImage(file)
          url = uploaded.url
          uploadedCount.value += 1
          if (hash) {
            seen.set(hash, url)
            cache[hash] = url
          }
        } else {
          reusedCount.value += 1
        }
        map[image.name] = url
        if (image.path) map[image.path] = url
      } catch (err) {
        failures.push(image.name)
      }
    }
    saveImageCache(cache)
    rawMarkdown.value = replaceImagePaths(rawMarkdown.value, map)
    chapters.value = chapters.value.map((chapter) => ({
      ...chapter,
      content: replaceImagePaths(chapter.content, map)
    }))
    if (failures.length) {
      toast(`有 ${failures.length} 张图片上传失败：${failures.slice(0, 3).join('、')}，请重试`)
      return false
    }
    imagesUploaded.value = true
    const processed = uploadedCount.value + reusedCount.value
    if (reusedCount.value) {
      toast(`已处理 ${processed} 张图片，其中 ${reusedCount.value} 张直接复用`)
    } else {
      toast(`已上传 ${uploadedCount.value} 张图片并替换路径`)
    }
    return true
  } catch (err) {
    toastError(err)
    return false
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

async function loadReference(id) {
  referenceContent.value = ''
  referenceTitle.value = ''
  if (!id) return true
  referenceLoading.value = true
  try {
    const data = await api.get(`/docs/${id}`)
    const doc = data.doc || {}
    referenceContent.value = doc.content || ''
    referenceTitle.value = doc.title || ''
    localStorage.setItem('folio.publish.reference', String(id))
    return Boolean(referenceContent.value)
  } catch (err) {
    toastError(err)
    return false
  } finally {
    referenceLoading.value = false
  }
}

watch(referenceID, (id) => {
  if (!id) {
    referenceContent.value = ''
    referenceTitle.value = ''
    localStorage.removeItem('folio.publish.reference')
    return
  }
  loadReference(id)
})

async function formatMarkdown() {
  const targets = chapters.value.length > 1
    ? chapters.value.map((chapter) => chapter.content)
    : [rawMarkdown.value]
  if (!targets.some((item) => item.trim())) {
    toast('先导入或粘贴正文')
    return
  }
  const ok = await confirmDialog({
    title: 'Markdown 排版',
    message: chapters.value.length > 1
      ? `AI 会依次整理 ${chapters.value.length} 个章节的标题层级、空行、列表和链接语法，不改写正文。`
      : 'AI 会整理标题层级、空行、列表、代码围栏和链接语法，不改写正文。应用后可以撤销。',
    confirmText: '开始排版'
  })
  if (!ok) return
  markdownFormatting.value = true
  try {
    const results = []
    for (const content of targets) {
      if (!content.trim()) {
        results.push(content)
        continue
      }
      const data = await api.post('/admin/markdown/format', { content })
      results.push(data.markdown || content)
    }
    const backup = {
      raw: rawMarkdown.value,
      chapters: chapters.value.map((chapter) => chapter.content)
    }
    if (chapters.value.length > 1) {
      chapters.value = chapters.value.map((chapter, index) => ({ ...chapter, content: results[index] }))
      rawMarkdown.value = results[0]
    } else {
      rawMarkdown.value = results[0]
    }
    markdownBackup.value = backup
    toast(chapters.value.length > 1 ? `已排版 ${results.length} 个章节` : 'Markdown 已排版，可撤销')
  } catch (err) {
    toastError(err)
  } finally {
    markdownFormatting.value = false
  }
}

function undoMarkdownFormat() {
  if (!markdownBackup.value) return
  rawMarkdown.value = markdownBackup.value.raw
  chapters.value = chapters.value.map((chapter, index) => ({
    ...chapter,
    content: markdownBackup.value.chapters[index] ?? chapter.content
  }))
  markdownBackup.value = null
  toast('已撤销排版')
}

async function review() {
  if (!rawMarkdown.value.trim()) {
    toast('先导入或粘贴正文')
    return
  }
  if (referenceID.value && !referenceContent.value) {
    const ok = await loadReference(referenceID.value)
    if (!ok) {
      toast('参考文档读取失败，请换一篇或取消参考')
      return
    }
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
      kind: targetKind.value,
      reference: referenceContent.value,
      referenceTitle: referenceTitle.value
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
    (item) => sameTitle(item.title, clean) && (parent ? item.parentID === parent : !item.parentID)
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
  if (uploading.value) {
    toast('图片还在上传，请稍候')
    return
  }
  if (pendingImages.value) {
    toast('还有图片是相对路径，请先上传图片后再发布')
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
    const savedReference = Number(localStorage.getItem('folio.publish.reference') || 0)
    if (savedReference && allDocs.value.some((item) => item.id === savedReference)) {
      referenceID.value = savedReference
    }
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
          <button class="btn btn-ghost btn-sm" type="button" :disabled="markdownFormatting" @click="formatMarkdown">
            {{ markdownFormatting ? '排版中…' : 'Markdown 排版' }}
          </button>
          <button v-if="markdownBackup" class="btn btn-ghost btn-sm" type="button" @click="undoMarkdownFormat">撤销排版</button>
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
            <label>导入文件</label>
            <label class="publish-file-picker" for="publish-file" @dragover.prevent @drop.prevent="onDrop">
              <input id="publish-file" class="publish-file-input" type="file" accept=".md,.zip" @change="onFile" />
              <span class="publish-file-icon"><Icon name="cloud" :size="22" /></span>
              <span class="publish-file-text">
                <strong>{{ fileName || '选择 Markdown 或 zip 文件' }}</strong>
                <small>点击选择，或把文件拖到这里</small>
              </span>
              <span class="publish-file-action">选择文件</span>
            </label>
            <p class="hint">zip 支持 book.md + chapters/*.md + images/*，会自动读取章节和图片。</p>
          </div>
          <div class="field">
            <label for="publish-content">正文 Markdown</label>
            <textarea id="publish-content" v-model="rawMarkdown" rows="20" placeholder="粘贴 Markdown，或上传 .md / .zip"></textarea>
          </div>
          <div v-if="zipImages.length" class="publish-images">
            <p>
              {{ imagesUploaded
                ? `已处理 ${uploadedCount + reusedCount} 张图片${reusedCount ? `（复用 ${reusedCount} 张）` : ''}`
                : `检测到 ${zipImages.length} 张图片` }}
            </p>
            <button class="btn btn-ghost btn-sm" type="button" :disabled="uploading" @click="uploadZipImages">
              {{ uploading ? `上传中 ${uploadedCount + reusedCount}/${zipImages.length}…` : imagesUploaded ? '重新上传图片' : '上传图片并替换路径' }}
            </button>
          </div>
          <p v-if="pendingImages && !uploading" class="hint publish-warn">
            还有图片是相对路径，发布会失败。请确认 zip 里的图片已上传，或把图片改成完整 URL。
          </p>
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
                <label for="publish-root">顶级文档 <span class="hint">输入或选择已有</span></label>
                <div class="suggest-control">
                  <input
                    id="publish-root"
                    v-model="rootTitle"
                    placeholder="读书笔记"
                    autocomplete="off"
                    @focus="rootFocus = true"
                    @blur="rootFocus = false"
                    @keydown.esc="rootFocus = false"
                  />
                  <div v-if="rootFocus && rootSuggestions.length" class="doc-suggest">
                    <button
                      v-for="doc in rootSuggestions"
                      :key="doc.id"
                      class="doc-suggest-item"
                      type="button"
                      @mousedown.prevent="pickRoot(doc)"
                    >
                      <span>{{ doc.title }}</span>
                      <small>{{ doc.status === 'draft' ? '草稿' : '已有' }}</small>
                    </button>
                  </div>
                </div>
                <p class="hint">{{ rootHint }}</p>
              </div>
              <div class="field">
                <label for="publish-section">上级文档 <span class="hint">输入或选择已有</span></label>
                <div class="suggest-control">
                  <input
                    id="publish-section"
                    v-model="sectionTitle"
                    :placeholder="bookTitle || '认知觉醒'"
                    autocomplete="off"
                    @focus="sectionFocus = true"
                    @blur="sectionFocus = false"
                    @keydown.esc="sectionFocus = false"
                  />
                  <div v-if="sectionFocus && sectionSuggestions.length" class="doc-suggest">
                    <button
                      v-for="doc in sectionSuggestions"
                      :key="doc.id"
                      class="doc-suggest-item"
                      type="button"
                      @mousedown.prevent="pickSection(doc)"
                    >
                      <span>{{ doc.title }}</span>
                      <small>{{ doc.status === 'draft' ? '草稿' : parentTitle(doc) }}</small>
                    </button>
                  </div>
                </div>
                <p class="hint">{{ sectionHint }}</p>
              </div>
              <p class="hint">输入已有标题会自动复用文档；选择其他分区的上级文档时，会自动补全顶级文档。</p>
            </template>
            <div v-else class="field">
              <label>上级文档</label>
              <SelectBox v-model="parentID" :options="parentOptions" placeholder="无（顶级文档）" />
            </div>
            <label class="checkbox">
              <input v-model="searchHidden" type="checkbox" />
              在搜索和文档列表中隐藏（仅作为目录节点）
            </label>
          </template>

          <div class="field">
            <label>状态</label>
            <SelectBox v-model="status" :options="statusOptions" />
          </div>
          <div class="field">
            <label>参考文风 <span class="hint">可选</span></label>
            <SelectBox v-model="referenceID" :options="referenceOptions" placeholder="不参考" />
            <p class="hint">
              {{ referenceLoading ? '正在读取参考文档…' : referenceTitle ? `参考：${referenceTitle}` : '选一篇已发布文档，AI 会学习它的句长、节奏和用词。' }}
            </p>
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
