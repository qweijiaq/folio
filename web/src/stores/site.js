import { reactive } from 'vue'
import { api } from '../api/client'

export const site = reactive({
  name: '知微',
  description: '一个人的知识花园，存放文章、手册与零散的思考。',
  owner: 'weijia',
  year: new Date().getFullYear(),
  loaded: false
})

export async function loadSite() {
  if (site.loaded) return site
  try {
    const data = await api.get('/site')
    Object.assign(site, data.site)
    site.loaded = true
  } catch {
    /* 使用默认文案 */
  }
  return site
}

export function mergeSite(info) {
  if (!info) return
  Object.assign(site, info, { loaded: true })
}
