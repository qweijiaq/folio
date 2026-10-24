<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api/client'
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
const providerId = ref('')
const apiKey = ref('')
const model = ref('')
const customModel = ref('')
const question = ref('')
const askedQuestion = ref('')
const answer = ref('')
const sources = ref([])
const loading = ref(false)
const providersLoading = ref(true)
const providersError = ref('')
const error = ref('')
const answerModel = ref('')
const answerProvider = ref('')

const examples = [
  '这个站点是怎么统一博客和文档的？',
  '有哪些关于手写 CSS 的内容？',
  'Go 服务端有哪些约定？'
]

const currentProvider = computed(() => providers.value.find((item) => item.id === providerId.value) || null)
const selectedModel = computed(() => (model.value === CUSTOM_MODEL ? customModel.value.trim() : model.value))

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

function resetForProvider(id) {
  const provider = providers.value.find((item) => item.id === id)
  providerId.value = id
  apiKey.value = readKey(id)
  model.value = provider?.models?.[0]?.id || ''
  customModel.value = ''
  saveProviderPref(id)
}

function onProviderChange(event) {
  storeKey(providerId.value, apiKey.value)
  error.value = ''
  resetForProvider(event.target.value)
  answer.value = ''
  sources.value = []
  askedQuestion.value = ''
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

watch(apiKey, (value) => storeKey(providerId.value, value))

onMounted(loadProviders)

function useExample(text) {
  question.value = text
}

function clearKey() {
  apiKey.value = ''
  storeKey(providerId.value, '')
  toast('已清除当前标签页的 API Key')
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
  if (!key) {
    error.value = `请先填写你自己的 ${name} API Key`
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
  storeKey(providerId.value, key)
  loading.value = true
  answer.value = ''
  sources.value = []
  try {
    const data = await api.post('/ai/ask', {
      question: text,
      apiKey: key,
      provider: providerId.value,
      model: chosenModel
    })
    answer.value = data.answer || ''
    sources.value = data.sources || []
    askedQuestion.value = text
    answerModel.value = data.model || chosenModel
    answerProvider.value = data.provider || providerId.value
  } catch (err) {
    error.value = err.message || 'AI 问答失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page ask-page">
    <header class="page-hero container">
      <span class="eyebrow">检索 · 提问</span>
      <h1>问 AI</h1>
      <p class="page-lede">向这个站点里的文章和文档提问。AI 只根据站内资料回答，并给出引用。</p>
    </header>

    <div class="container ask-layout">
      <section class="panel ask-form-panel">
        <header class="panel-head">
          <h2>你的问题</h2>
          <p>API Key 只保存在当前浏览器标签页，提交时用于调用所选服务商，不会写入数据库或日志。</p>
        </header>

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
                :placeholder="currentProvider?.keyHint || 'sk-...'"
              />
              <button v-if="apiKey" class="text-link" type="button" @click="clearKey">清除</button>
            </div>
            <span class="hint">
              只保存在当前标签页，并按服务商分别记忆。
              <a v-if="currentProvider?.consoleURL" :href="currentProvider.consoleURL" target="_blank" rel="noopener noreferrer">获取 Key</a>
            </span>
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
            <label for="ask-question">问题</label>
            <textarea
              id="ask-question"
              v-model="question"
              rows="5"
              maxlength="500"
              placeholder="例如：这个站点是怎么统一博客和文档的？"
            ></textarea>
            <span class="hint">{{ question.length }} / 500 · 会先从站内内容中检索相关资料</span>
          </div>

          <div class="ask-examples">
            <span>试试：</span>
            <button v-for="text in examples" :key="text" type="button" class="tag" @click="useExample(text)">{{ text }}</button>
          </div>

          <p v-if="error" class="form-error">{{ error }}</p>

          <button class="btn btn-primary btn-block" type="submit" :disabled="loading || providersLoading || !currentProvider">
            <Icon name="spark" :size="16" />
            {{ loading ? '正在检索并回答…' : '提问' }}
          </button>
        </form>
      </section>

      <section class="panel ask-answer-panel">
        <header class="panel-head">
          <h2>回答</h2>
          <p v-if="answerModel">由 {{ providerName(answerProvider) }} · {{ answerModel }} 生成，仅供阅读参考。</p>
          <p v-else>回答会显示在这里。</p>
        </header>

        <p v-if="loading" class="muted loading-line">正在检索资料并组织回答…</p>
        <template v-else-if="answer">
          <div class="ask-question-echo">{{ askedQuestion }}</div>
          <div class="ask-answer">
            <MarkdownView :source="answer" />
          </div>
          <section v-if="sources.length" class="ask-sources">
            <h3>引用资料</h3>
            <RouterLink v-for="source in sources" :key="source.id" :to="source.url" class="ask-source">
              <span class="badge">{{ source.kind === 'doc' ? '文档' : '文章' }}</span>
              <span>{{ source.title }}</span>
              <Icon name="arrowRight" :size="14" />
            </RouterLink>
          </section>
        </template>
        <EmptyState v-else icon="spark" title="还没有回答" description="选择服务商，填写 Key 和问题，AI 会在站内资料中查找答案。" />
      </section>
    </div>
  </div>
</template>
