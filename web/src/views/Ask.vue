<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api/client'
import { toast } from '../stores/toast'
import { usePageMeta } from '../composables/usePageMeta'
import MarkdownView from '../components/MarkdownView.vue'
import PasswordInput from '../components/PasswordInput.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'

usePageMeta(() => ({ title: '问 AI', description: '用自己的 API Key 向知微知识库提问', noindex: true }))

const KEY_STORAGE = 'folio.ai.key'
const apiKey = ref('')
const model = ref('claude-sonnet-5-5')
const question = ref('')
const askedQuestion = ref('')
const answer = ref('')
const sources = ref([])
const loading = ref(false)
const error = ref('')
const answerModel = ref('')

const examples = [
  '这个站点是怎么统一博客和文档的？',
  '有哪些关于手写 CSS 的内容？',
  'Go 服务端有哪些约定？'
]

onMounted(() => {
  try {
    apiKey.value = sessionStorage.getItem(KEY_STORAGE) || ''
  } catch {
    apiKey.value = ''
  }
})

function useExample(text) {
  question.value = text
}

function clearKey() {
  apiKey.value = ''
  try {
    sessionStorage.removeItem(KEY_STORAGE)
  } catch {
    /* ignore */
  }
  toast('已清除当前标签页的 API Key')
}

async function ask() {
  error.value = ''
  const key = apiKey.value.trim()
  const text = question.value.trim()
  if (!key) {
    error.value = '请先填写你自己的 Anthropic API Key'
    return
  }
  if (!text) {
    error.value = '请先写下问题'
    return
  }
  try {
    sessionStorage.setItem(KEY_STORAGE, key)
  } catch {
    /* 隐私模式下可能不可用，不影响本次提问 */
  }
  loading.value = true
  answer.value = ''
  sources.value = []
  try {
    const data = await api.post('/ai/ask', { question: text, apiKey: key, model: model.value })
    answer.value = data.answer || ''
    sources.value = data.sources || []
    askedQuestion.value = text
    answerModel.value = data.model || model.value
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
          <p>API Key 只保存在当前浏览器标签页，提交时用于调用 Anthropic，不会写入数据库或日志。</p>
        </header>

        <form class="ask-form" @submit.prevent="ask">
          <div class="field">
            <label for="ask-key">Anthropic API Key</label>
            <div class="ask-key-row">
              <PasswordInput id="ask-key" v-model="apiKey" autocomplete="off" placeholder="sk-ant-..." />
              <button v-if="apiKey" class="text-link" type="button" @click="clearKey">清除</button>
            </div>
            <span class="hint">本站不提供 Key。使用你自己的 Anthropic API Key，费用由你的账号承担。</span>
          </div>

          <div class="field">
            <label for="ask-model">模型</label>
            <select id="ask-model" v-model="model">
              <option value="claude-sonnet-5-5">Claude Sonnet 5.5（更快、更省）</option>
              <option value="claude-opus-5-5">Claude Opus 5.5（更强）</option>
              <option value="claude-haiku-5-5">Claude Haiku 5.5（最轻量）</option>
            </select>
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

          <button class="btn btn-primary btn-block" type="submit" :disabled="loading">
            <Icon name="spark" :size="16" />
            {{ loading ? '正在检索并回答…' : '提问' }}
          </button>
        </form>
      </section>

      <section class="panel ask-answer-panel">
        <header class="panel-head">
          <h2>回答</h2>
          <p v-if="answerModel">由 {{ answerModel }} 生成，仅供阅读参考。</p>
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
        <EmptyState v-else icon="spark" title="还没有回答" description="填写 Key 和问题，AI 会在站内资料中查找答案。" />
      </section>
    </div>
  </div>
</template>
