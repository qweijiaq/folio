<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, getToken } from '../api/client'
import { toast } from '../stores/toast'
import { usePageMeta } from '../composables/usePageMeta'
import MarkdownView from '../components/MarkdownView.vue'
import PasswordInput from '../components/PasswordInput.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'

usePageMeta(() => ({ title: '问 AI', description: '用自己的 API Key 向知微知识库提问', noindex: true }))

const KEY_PREFIX = 'folio.ai.key.'
const PROVIDER_STORAGE = 'folio.ai.provider'
const CUSTOM_MODEL = '__custom__'

const providers = ref([])
const aiMeta = ref(null)
const savedKeys = ref({})
const rememberKey = ref(true)
const providerId = ref('')
const apiKey = ref('')
const model = ref('')
const customModel = ref('')
const mode = ref('strict')
const question = ref('')
const thread = ref([])
const loading = ref(false)
const loadingStage = ref('')
const providersLoading = ref(true)
const providersError = ref('')
const error = ref('')
const threadEl = ref(null)
let abortController = null

const examples = [
  '这个站点怎么发布一篇文章？',
  '邀请注册和公开注册有什么区别？',
  '私信、收藏和关注的隐私怎么设置？',
  '站点的七牛云存储怎么配置？'
]

const currentProvider = computed(() => providers.value.find((item) => item.id === providerId.value) || null)
const selectedModel = computed(() => (model.value === CUSTOM_MODEL ? customModel.value.trim() : model.value))
const knowledge = computed(() => aiMeta.value?.knowledge || {})
const hasSavedKey = computed(() => Boolean(savedKeys.value[providerId.value]))

function readKey(id) {
  try {
    return sessionStorage.getItem(KEY_PREFIX + id) || ''
  } catch {
    return ''
  }
}

function storeKey(id, value) {
  if (!id) return
  try {
    if (value.trim()) sessionStorage.setItem(KEY_PREFIX + id, value.trim())
    else sessionStorage.removeItem(KEY_PREFIX + id)
  } catch {
    /* 隐私模式下可能不可用，不影响本次提问 */
  }
}

function readProviderPref() {
  try {
    return localStorage.getItem(PROVIDER_STORAGE) || ''
  } catch {
    return ''
  }
}

function saveProviderPref(id) {
  try {
    localStorage.setItem(PROVIDER_STORAGE, id)
  } catch {
    /* ignore */
  }
}

function providerName(id) {
  return providers.value.find((item) => item.id === id)?.name || id
}

function modeLabel(value) {
  return value === 'expanded' ? '通用知识' : '严格站内'
}

function sourceKindLabel(kind) {
  if (kind === 'help') return '站点帮助'
  if (kind === 'doc') return '文档'
  return '文章'
}

function resetForProvider(id) {
  const provider = providers.value.find((item) => item.id === id)
  providerId.value = id
  apiKey.value = readKey(id)
  model.value = provider?.models?.[0]?.id || ''
  customModel.value = ''
  rememberKey.value = true
  saveProviderPref(id)
}

function onProviderChange(event) {
  storeKey(providerId.value, apiKey.value)
  error.value = ''
  resetForProvider(event.target.value)
}

async function loadProviders() {
  try {
    const data = await api.get('/ai/providers')
    providers.value = data.providers || []
    if (!providers.value.length) {
      providersError.value = '暂时没有可用的 AI 服务商'
      return
    }
    const preferred = readProviderPref()
    const initial = providers.value.find((item) => item.id === preferred) || providers.value[0]
    resetForProvider(initial.id)
  } catch (err) {
    providersError.value = err.message || '加载 AI 服务商失败'
  } finally {
    providersLoading.value = false
  }
}

async function loadMeta() {
  try {
    aiMeta.value = await api.get('/ai/meta')
  } catch {
    /* 知识库信息加载失败不影响提问 */
  }
}

async function loadSavedKeys() {
  try {
    const data = await api.get('/ai/keys')
    savedKeys.value = data.saved || {}
    if (hasSavedKey.value) rememberKey.value = true
  } catch {
    savedKeys.value = {}
  }
}

watch(apiKey, (value) => storeKey(providerId.value, value))

onMounted(async () => {
  await loadProviders()
  await Promise.all([loadMeta(), loadSavedKeys()])
})

function useExample(text) {
  question.value = text
  nextTick(() => document.querySelector('#ask-question')?.focus())
}

async function clearKey() {
  const provider = providerId.value
  apiKey.value = ''
  storeKey(provider, '')
  if (savedKeys.value[provider]) {
    try {
      await api.del(`/ai/keys/${encodeURIComponent(provider)}`)
      savedKeys.value = { ...savedKeys.value, [provider]: false }
      toast('已清除保存的 API Key')
      return
    } catch (err) {
      toast(err.message || '清除保存的 Key 失败', 'error')
      return
    }
  }
  toast('已清除当前标签页的 API Key')
}

function buildHistory() {
  return thread.value
    .filter((item) => item.role === 'user' || item.role === 'assistant')
    .filter((item) => !item.pending && item.content?.trim())
    .slice(-6)
    .map((item) => ({ role: item.role, content: item.content }))
}

async function scrollThread() {
  await nextTick()
  const el = threadEl.value
  if (el) el.scrollTop = el.scrollHeight
}

function applyStreamEvent(raw, assistant) {
  let event = 'message'
  const dataLines = []
  for (const line of raw.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (!dataLines.length) return
  let payload
  try {
    payload = JSON.parse(dataLines.join('\n'))
  } catch {
    return
  }
  if (event === 'meta') {
    assistant.sources = payload.sources || []
    assistant.provider = payload.provider || assistant.provider
    assistant.model = payload.model || assistant.model
    assistant.mode = payload.mode || assistant.mode
    assistant.retrieved = payload.retrieved || 0
    loadingStage.value = assistant.retrieved
      ? `已找到 ${assistant.retrieved} 段资料，正在生成回答…`
      : '正在生成回答…'
  } else if (event === 'delta') {
    assistant.content += payload.text || ''
    loadingStage.value = '正在生成回答…'
    scrollThread()
  } else if (event === 'done') {
    if (!assistant.content && payload.answer) assistant.content = payload.answer
    assistant.pending = false
    loadingStage.value = ''
  } else if (event === 'error') {
    assistant.error = payload.error || '生成回答失败'
    assistant.pending = false
    loadingStage.value = ''
  }
}

async function readStream(response, assistant) {
  const reader = response.body?.getReader()
  if (!reader) throw new Error('当前浏览器不支持流式回答')
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
    let split = buffer.indexOf('\n\n')
    while (split >= 0) {
      const raw = buffer.slice(0, split)
      buffer = buffer.slice(split + 2)
      applyStreamEvent(raw, assistant)
      split = buffer.indexOf('\n\n')
    }
  }
  if (buffer.trim()) applyStreamEvent(buffer, assistant)
}

async function ask() {
  error.value = ''
  const key = apiKey.value.trim()
  const text = question.value.trim()
  const name = currentProvider.value?.name || '所选服务商'
  const chosenModel = selectedModel.value
  if (!providers.value.length) {
    error.value = providersError.value || 'AI 服务商还没有加载完成'
    return
  }
  if (!key && !hasSavedKey.value) {
    error.value = `请先填写你自己的 ${name} API Key，或勾选「记住 Key」保存一次`
    return
  }
  if (!text) {
    error.value = '请先写下问题'
    return
  }
  if (!chosenModel) {
    error.value = '请填写要使用的模型名称'
    return
  }
  if (key) {
    storeKey(providerId.value, key)
    if (rememberKey.value) {
      try {
        await api.put('/ai/keys', { provider: providerId.value, apiKey: key })
        savedKeys.value = { ...savedKeys.value, [providerId.value]: true }
        apiKey.value = ''
      } catch (err) {
        toast(err.message || 'Key 保存失败，本次仍会使用你输入的 Key', 'error')
      }
    }
  }
  const history = buildHistory()
  thread.value.push({ role: 'user', content: text })
  const assistant = {
    role: 'assistant',
    content: '',
    sources: [],
    provider: providerId.value,
    model: chosenModel,
    mode: mode.value,
    retrieved: 0,
    pending: true,
    error: ''
  }
  thread.value.push(assistant)
  question.value = ''
  loading.value = true
  loadingStage.value = '正在检索站内资料…'
  scrollThread()

  abortController = new AbortController()
  try {
    const response = await fetch('/api/ai/ask/stream', {
      method: 'POST',
      headers: {
        Accept: 'text/event-stream',
        'Content-Type': 'application/json',
        Authorization: `Bearer ${getToken()}`
      },
      body: JSON.stringify({
        question: text,
        apiKey: key,
        provider: providerId.value,
        model: chosenModel,
        mode: mode.value,
        history
      }),
      signal: abortController.signal
    })
    if (!response.ok) {
      let message = 'AI 问答失败'
      try {
        const data = await response.json()
        message = data.error || message
      } catch {
        /* ignore */
      }
      throw new Error(message)
    }
    await readStream(response, assistant)
    if (!assistant.content && !assistant.error) {
      assistant.error = '服务商没有返回有效回答'
    }
  } catch (err) {
    if (err.name === 'AbortError') {
      assistant.content = assistant.content || ''
      assistant.error = assistant.error || '已停止生成'
    } else {
      assistant.error = err.message || 'AI 问答失败'
    }
  } finally {
    assistant.pending = false
    loading.value = false
    loadingStage.value = ''
    abortController = null
    scrollThread()
  }
}

function stopAsk() {
  if (abortController) abortController.abort()
}

function clearThread() {
  if (abortController) abortController.abort()
  thread.value = []
  error.value = ''
  loading.value = false
  loadingStage.value = ''
}

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
})
</script>

<template>
  <div class="page ask-page">
    <header class="page-hero container">
      <span class="eyebrow">检索 · 提问</span>
      <h1>问 AI</h1>
      <p class="page-lede">向这个站点里的文章、文档和功能说明提问。AI 先检索站内资料，再结合你选择的服务商回答。</p>
    </header>

    <div class="container ask-layout">
      <section class="panel ask-form-panel">
        <header class="panel-head">
          <div class="ask-panel-title">
            <h2>对话</h2>
            <button v-if="thread.length" class="text-link" type="button" @click="clearThread">清空对话</button>
          </div>
          <p>API Key 只保存在当前浏览器标签页，提交时用于调用所选服务商，不会写入数据库或日志。</p>
        </header>

        <div v-if="aiMeta" class="ask-coverage">
          <span class="ask-coverage-label">知识库覆盖</span>
          <span>{{ knowledge.articles || 0 }} 篇文章 · {{ knowledge.docs || 0 }} 篇文档 · {{ knowledge.help || 0 }} 条站点帮助</span>
          <RouterLink to="/about">查看关于页</RouterLink>
        </div>

        <form class="ask-form" @submit.prevent="ask">
          <div class="field">
            <label for="ask-provider">服务商</label>
            <select
              id="ask-provider"
              :value="providerId"
              :disabled="providersLoading || !providers.length"
              @change="onProviderChange"
            >
              <option v-for="provider in providers" :key="provider.id" :value="provider.id">
                {{ provider.name }}
              </option>
            </select>
            <span v-if="providersLoading" class="hint">正在加载服务商…</span>
            <span v-else-if="providersError" class="form-error">{{ providersError }}</span>
            <span v-else class="hint">本站不提供 Key。请选择你有账号的服务商，费用由你的账号承担。</span>
          </div>

          <div class="field">
            <label for="ask-key">{{ currentProvider?.name || '' }} API Key</label>
            <div class="ask-key-row">
              <PasswordInput
                id="ask-key"
                v-model="apiKey"
                autocomplete="off"
                :placeholder="hasSavedKey ? '已保存，可留空直接提问' : (currentProvider?.keyHint || 'sk-...')"
              />
              <button v-if="apiKey || hasSavedKey" class="text-link" type="button" @click="clearKey">清除</button>
            </div>
            <span v-if="hasSavedKey" class="hint ask-key-saved">
              <Icon name="check" :size="13" />
              已加密保存在本站，可留空直接提问；输入新 Key 会覆盖。
              <a v-if="currentProvider?.consoleURL" :href="currentProvider.consoleURL" target="_blank" rel="noopener noreferrer">获取 Key</a>
            </span>
            <span v-else class="hint">
              只保存在当前标签页；勾选下方选项后才会加密保存到本站。
              <a v-if="currentProvider?.consoleURL" :href="currentProvider.consoleURL" target="_blank" rel="noopener noreferrer">获取 Key</a>
            </span>
            <label class="ask-remember">
              <input v-model="rememberKey" type="checkbox" />
              <span>记住 Key，使用 FOLIO_SECRET 加密保存，可随时清除</span>
            </label>
          </div>

          <div class="field">
            <label for="ask-model">模型</label>
            <select id="ask-model" v-model="model" :disabled="!currentProvider">
              <option v-for="item in currentProvider?.models || []" :key="item.id" :value="item.id">
                {{ item.name }}{{ item.note ? `（${item.note}）` : '' }}
              </option>
              <option :value="CUSTOM_MODEL">其他模型（手动输入）</option>
            </select>
            <input
              v-if="model === CUSTOM_MODEL"
              v-model="customModel"
              class="ask-custom-model"
              type="text"
              maxlength="100"
              placeholder="例如 deepseek-chat"
              aria-label="自定义模型名称"
            />
          </div>

          <div class="field">
            <span class="ask-field-label">回答模式</span>
            <div class="ask-mode" role="group" aria-label="回答模式">
              <button type="button" :class="{ active: mode === 'strict' }" @click="mode = 'strict'">
                严格站内
                <small>只依据站内资料</small>
              </button>
              <button type="button" :class="{ active: mode === 'expanded' }" @click="mode = 'expanded'">
                通用知识
                <small>不足时补充站外常识</small>
              </button>
            </div>
          </div>

          <div class="field">
            <label for="ask-question">{{ thread.length ? '继续追问' : '问题' }}</label>
            <textarea
              id="ask-question"
              v-model="question"
              rows="4"
              maxlength="500"
              :placeholder="thread.length ? '基于上面的回答继续问…' : '例如：这个站点怎么发布一篇文章？'"
            ></textarea>
            <span class="hint">{{ question.length }} / 500 · 会先从站内内容中检索相关资料</span>
          </div>

          <div v-if="!thread.length" class="ask-examples">
            <span>试试：</span>
            <button v-for="text in examples" :key="text" type="button" class="tag" @click="useExample(text)">{{ text }}</button>
          </div>

          <p v-if="error" class="form-error">{{ error }}</p>

          <div class="ask-submit-row">
            <button class="btn btn-primary btn-block" type="submit" :disabled="loading || providersLoading || !currentProvider">
              <Icon name="spark" :size="16" />
              {{ loading ? '正在回答…' : thread.length ? '继续提问' : '提问' }}
            </button>
            <button v-if="loading" class="btn btn-ghost btn-sm ask-stop" type="button" @click="stopAsk">停止</button>
          </div>
        </form>
      </section>

      <section class="panel ask-answer-panel">
        <header class="panel-head">
          <div class="ask-panel-title">
            <h2>回答</h2>
            <span v-if="thread.length" class="hint">{{ thread.filter((item) => item.role === 'user').length }} 轮对话</span>
          </div>
          <p v-if="loading">{{ loadingStage }}</p>
          <p v-else>回答会显示在这里，每轮都会重新检索站内资料。</p>
        </header>

        <div v-if="thread.length" ref="threadEl" class="ask-thread">
          <div v-for="(message, index) in thread" :key="index" class="ask-message" :class="`ask-message--${message.role}`">
            <template v-if="message.role === 'user'">
              <div class="ask-user-bubble">{{ message.content }}</div>
            </template>
            <template v-else>
              <div class="ask-answer-meta">
                <span class="badge">{{ modeLabel(message.mode) }}</span>
                <span v-if="message.provider">{{ providerName(message.provider) }} · {{ message.model }}</span>
                <span v-if="message.retrieved"> · {{ message.retrieved }} 个来源</span>
              </div>
              <p v-if="message.pending && !message.content" class="muted loading-line">{{ loadingStage || '正在生成回答…' }}</p>
              <div v-if="message.content" class="ask-answer">
                <MarkdownView :source="message.content" />
              </div>
              <p v-if="message.error" class="form-error">{{ message.error }}</p>
              <section v-if="message.sources?.length" class="ask-sources">
                <h3>引用资料</h3>
                <details v-for="source in message.sources" :key="source.ref" class="ask-source-item">
                  <summary>
                    <span class="badge">{{ sourceKindLabel(source.kind) }}</span>
                    <span class="ask-source-title">{{ source.title }}</span>
                    <span v-if="source.category" class="ask-source-category">{{ source.category }}</span>
                    <Icon name="chevronDown" :size="14" />
                  </summary>
                  <p class="ask-source-excerpt">{{ source.excerpt }}</p>
                  <a :href="source.url" class="text-link">打开原文</a>
                </details>
              </section>
            </template>
          </div>
        </div>

        <p v-else-if="loading" class="muted loading-line">正在检索资料并组织回答…</p>
        <EmptyState
          v-else
          icon="spark"
          title="还没有回答"
          description="选择服务商和回答模式，填写 Key 后提问。也可以先去关于页了解站点功能。"
        />
      </section>
    </div>
  </div>
</template>
