import { ref } from 'vue'
import { api } from '../api/client'

let cache = null
let pending = null

export function useDocLinks() {
  const linkMap = ref(cache || {})

  async function loadDocLinks() {
    if (cache) {
      linkMap.value = cache
      return cache
    }
    if (!pending) {
      pending = api
        .get('/doc-links')
        .then((data) => {
          const map = {}
          for (const item of data.items || []) {
            if (!item?.title) continue
            map[item.title.trim().toLowerCase()] = item
          }
          cache = map
          return map
        })
        .catch(() => null)
        .finally(() => {
          pending = null
        })
    }
    const map = await pending
    if (map) linkMap.value = map
    return map
  }

  return { linkMap, loadDocLinks }
}
