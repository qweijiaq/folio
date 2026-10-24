<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api/client'
import { auth } from '../stores/auth'
import { toast, toastError } from '../stores/toast'
import { formatDate, roleLabel } from '../utils/format'
import CoverImage from '../components/CoverImage.vue'
import UserAvatar from '../components/UserAvatar.vue'
import DocCard from '../components/DocCard.vue'
import EmptyState from '../components/EmptyState.vue'
import Icon from '../components/Icon.vue'
import { usePageMeta } from '../composables/usePageMeta'

const route = useRoute()
const router = useRouter()

const profile = ref(null)
const loading = ref(true)
const notFound = ref(false)
const tab = ref('articles')
const docs = ref([])
const people = ref([])
const tabLoading = ref(false)
const peopleHidden = ref(false)
const collectionFolders = ref([])
const collectionFolderID = ref(null)
const collectionDocs = ref([])
const collectionHidden = ref(false)

usePageMeta(() => {
  const current = profile.value
  if (!current) return { title: '读者主页', description: '知微读者主页' }
  const user = current.user
  const image = `/api/art/cover.png?seed=${encodeURIComponent(user.username)}&kind=user`
  return {
    title: user.nickname,
    description: user.bio || '知微读者主页',
    image,
    type: 'profile',
    url: `/u/${user.username}`,
    username: user.username,
    jsonLd: {
      '@context': 'https://schema.org',
      '@type': 'ProfilePage',
      mainEntity: {
        '@type': 'Person',
        name: user.nickname,
        alternateName: user.username,
        description: user.bio || '',
        url: `${window.location.origin}/u/${user.username}`,
        image: new URL(image, window.location.origin).href
      }
    }
  }
})

const username = computed(() => String(route.params.username || ''))
const isSelf = computed(() => profile.value?.isSelf)
const isFollowing = ref(false)

const settings = computed(() => profile.value?.settings || {
  openFollow: true,
  openFans: true,
  openPrivateChat: true,
  openCollect: false,
  homeStyle: 'cover'
})
const homeStyle = computed(() => ['cover', 'compact', 'plain'].includes(settings.value.homeStyle) ? settings.value.homeStyle : 'cover')
const tabs = computed(() => {
  const list = [
    { key: 'articles', label: '文章', count: profile.value?.counts?.articles ?? 0 },
    { key: 'docs', label: '文档', count: profile.value?.counts?.docs ?? 0 }
  ]
  if (profile.value?.isSelf || settings.value.openFollow) {
    list.push({ key: 'following', label: '关注', count: profile.value?.counts?.following ?? 0 })
  }
  if (profile.value?.isSelf || settings.value.openFans) {
    list.push({ key: 'followers', label: '粉丝', count: profile.value?.counts?.followers ?? 0 })
  }
  if (profile.value?.isSelf || settings.value.openCollect) {
    list.push({ key: 'collections', label: '收藏', count: profile.value?.counts?.favorites ?? 0 })
  }
  return list
})

async function loadProfile() {
  loading.value = true
  notFound.value = false
  try {
    const data = await api.get(`/users/${username.value}`)
    profile.value = data
    isFollowing.value = data.isFollowing
    tab.value = 'articles'
    await loadTab()
  } catch (err) {
    if (err.status === 404) notFound.value = true
    else toastError(err)
  } finally {
    loading.value = false
  }
}

async function loadTab() {
  if (!profile.value) return
  tabLoading.value = true
  docs.value = []
  people.value = []
  peopleHidden.value = false
  collectionDocs.value = []
  collectionHidden.value = false
  try {
    if (tab.value === 'articles' || tab.value === 'docs') {
      const data = await api.get('/docs', {
        author: username.value,
        kind: tab.value === 'articles' ? 'article' : 'doc',
        pageSize: 12
      })
      if (tab.value === 'articles' && profile.value.topDoc) {
        const filtered = data.items.filter((doc) => doc.id !== profile.value.topDoc.id)
        docs.value = filtered.length ? filtered : data.items
      } else {
        docs.value = data.items
      }
    } else if (tab.value === 'collections') {
      await loadCollections()
    } else {
      const data = await api.get(`/users/${username.value}/${tab.value}`, { pageSize: 24 })
      people.value = data.items
      peopleHidden.value = Boolean(data.hidden)
    }
  } catch (err) {
    toastError(err)
  } finally {
    tabLoading.value = false
  }
}

async function loadCollections() {
  const folderData = await api.get(`/users/${username.value}/favorite-folders`)
  if (folderData.hidden) {
    collectionHidden.value = true
    collectionFolders.value = []
    collectionDocs.value = []
    return
  }
  collectionFolders.value = folderData.items || []
  if (!collectionFolders.value.length) {
    collectionDocs.value = []
    return
  }
  if (!collectionFolders.value.some((folder) => folder.id === collectionFolderID.value)) {
    collectionFolderID.value = (collectionFolders.value.find((folder) => folder.isDefault) || collectionFolders.value[0]).id
  }
  await loadCollectionDocs(collectionFolderID.value)
}

async function loadCollectionDocs(folderID) {
  const data = await api.get(`/users/${username.value}/favorites`, { folderID, pageSize: 24 })
  collectionHidden.value = Boolean(data.hidden)
  collectionDocs.value = data.items || []
}

async function selectCollectionFolder(folder) {
  collectionFolderID.value = folder.id
  try {
    await loadCollectionDocs(folder.id)
  } catch (err) {
    toastError(err)
  }
}

function setTab(key) {
  if (tab.value === key) return
  tab.value = key
  loadTab()
}

async function toggleFollow() {
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  try {
    const data = isFollowing.value
      ? await api.del(`/users/${username.value}/follow`)
      : await api.post(`/users/${username.value}/follow`)
    isFollowing.value = data.following
    if (profile.value) profile.value.counts.followers = data.followers
    toast(data.following ? '已关注' : '已取消关注')
  } catch (err) {
    toastError(err)
  }
}

async function togglePerson(person) {
  if (!auth.user) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  try {
    const target = person.user.username
    const data = person.isFollowing
      ? await api.del(`/users/${target}/follow`)
      : await api.post(`/users/${target}/follow`)
    person.isFollowing = data.following
  } catch (err) {
    toastError(err)
  }
}

onMounted(loadProfile)
watch(username, loadProfile)
</script>

<template>
  <div class="page profile-page" :class="`profile-page--${homeStyle}`">
    <div v-if="loading" class="container page-loading">
      <p class="muted loading-line">正在加载主页…</p>
    </div>
    <div v-else-if="notFound" class="container page-loading">
      <EmptyState icon="user" title="找不到这位读者" description="用户名可能写错了，或者这个账号已经离开。">
        <RouterLink to="/" class="btn btn-ghost btn-sm">回到首页</RouterLink>
      </EmptyState>
    </div>

    <template v-else-if="profile">
      <div v-if="homeStyle !== 'plain'" class="profile-banner">
        <CoverImage :seed="profile.user.username" :title="profile.user.nickname" kind="user" />
        <div class="profile-banner-fade"></div>
      </div>

      <div class="container profile-head">
        <div class="profile-avatar">
          <UserAvatar :username="profile.user.username" :nickname="profile.user.nickname" :size="104" />
        </div>
        <div class="profile-identity">
          <div class="profile-name-row">
            <h1>{{ profile.user.nickname }}</h1>
            <span class="profile-handle">@{{ profile.user.username }}</span>
            <span v-if="['super_admin', 'admin'].includes(profile.user.role)" class="badge badge-admin">
              {{ roleLabel(profile.user.role) }}
            </span>
          </div>
          <p class="profile-bio">{{ profile.user.bio || '这个人还没有写简介。' }}</p>
          <p class="profile-joined">加入于 {{ formatDate(profile.user.createdAt) }}</p>
        </div>
        <div class="profile-actions">
          <RouterLink v-if="isSelf" to="/me" class="btn btn-ghost btn-sm">
            <Icon name="settings" :size="15" /> 编辑资料
          </RouterLink>
          <template v-else>
            <button class="btn btn-sm" :class="isFollowing ? 'btn-ghost' : 'btn-primary'" type="button" @click="toggleFollow">
              <Icon :name="isFollowing ? 'following' : 'follow'" :size="15" />
              {{ isFollowing ? '已关注' : '关注' }}
            </button>
            <RouterLink
              v-if="auth.user && settings.openPrivateChat"
              :to="{ path: '/messages', query: { tab: 'messages', with: profile.user.username } }"
              class="btn btn-ghost btn-sm"
            >
              <Icon name="message" :size="15" /> 私信
            </RouterLink>
            <span v-else-if="auth.user" class="profile-chat-closed">对方关闭了私信</span>
          </template>
        </div>
      </div>

      <div class="container profile-stats">
        <div><strong>{{ profile.counts.articles }}</strong><span>文章</span></div>
        <div><strong>{{ profile.counts.docs }}</strong><span>文档</span></div>
        <div><strong>{{ profile.isSelf || settings.openFollow ? profile.counts.following : '—' }}</strong><span>关注</span></div>
        <div><strong>{{ profile.isSelf || settings.openFans ? profile.counts.followers : '—' }}</strong><span>粉丝</span></div>
      </div>

      <section v-if="profile.topDoc" class="container profile-featured">
        <header class="section-head section-head--tight">
          <div>
            <span class="eyebrow">作者置顶</span>
            <h2>先读这一篇</h2>
          </div>
        </header>
        <DocCard :doc="profile.topDoc" variant="row" />
      </section>

      <div class="container profile-body">
        <div class="tabs">
          <button v-for="item in tabs" :key="item.key" type="button" :class="{ active: tab === item.key }" @click="setTab(item.key)">
            {{ item.label }} <span>{{ item.count }}</span>
          </button>
        </div>

        <p v-if="tabLoading" class="muted loading-line">正在加载…</p>

        <template v-else>
          <div v-if="tab === 'articles' || tab === 'docs'">
            <EmptyState
              v-if="!docs.length"
              icon="feather"
              :title="tab === 'articles' ? '还没有公开文章' : '还没有公开文档'"
              description="内容会在这里按时间排列。"
            />
            <div v-else class="doc-grid">
              <DocCard v-for="doc in docs" :key="doc.id" :doc="doc" />
            </div>
          </div>

          <div v-else-if="tab === 'collections'">
            <EmptyState
              v-if="collectionHidden"
              icon="eye"
              title="收藏没有公开"
              description="对方把收藏夹设为了私密。"
            />
            <template v-else-if="collectionFolders.length">
              <div class="folder-chips collection-chips">
                <button
                  v-for="folder in collectionFolders"
                  :key="folder.id"
                  class="tag"
                  :class="{ 'tag--active': collectionFolderID === folder.id }"
                  type="button"
                  @click="selectCollectionFolder(folder)"
                >
                  {{ folder.name }}
                  <span class="tag-count">{{ folder.count }}</span>
                </button>
              </div>
              <EmptyState
                v-if="!collectionDocs.length"
                icon="bookmark"
                title="这个收藏夹还是空的"
                description="对方还没有在这个收藏夹里放内容。"
              />
              <div v-else class="doc-grid collection-grid">
                <DocCard v-for="doc in collectionDocs" :key="doc.id" :doc="doc" />
              </div>
            </template>
            <EmptyState
              v-else
              icon="bookmark"
              title="还没有公开收藏"
              description="对方开启公开收藏后，收藏夹会出现在这里。"
            />
          </div>

          <div v-else>
            <EmptyState
              v-if="peopleHidden"
              icon="eye"
              title="这部分没有公开"
              description="对方把关注或粉丝列表设为了私密。"
            />
            <EmptyState
              v-else-if="!people.length"
              icon="user"
              :title="tab === 'following' ? '还没有关注任何人' : '还没有粉丝'"
              description="读者之间的连接会显示在这里。"
            />
            <div v-else class="people-table">
              <div v-for="person in people" :key="person.user.id" class="people-table-row">
                <RouterLink :to="`/u/${person.user.username}`" class="people-table-user">
                  <UserAvatar :username="person.user.username" :nickname="person.user.nickname" :size="42" />
                  <span>
                    <strong>{{ person.user.nickname }}</strong>
                    <span>{{ person.user.bio || `@${person.user.username}` }}</span>
                  </span>
                </RouterLink>
                <span class="people-table-count">{{ person.docCount }} 篇内容</span>
                <button
                  v-if="auth.user && auth.user.id !== person.user.id"
                  class="btn btn-sm"
                  :class="person.isFollowing ? 'btn-ghost' : 'btn-primary'"
                  type="button"
                  @click="togglePerson(person)"
                >
                  {{ person.isFollowing ? '已关注' : '关注' }}
                </button>
              </div>
            </div>
          </div>
        </template>
      </div>
    </template>
  </div>
</template>
