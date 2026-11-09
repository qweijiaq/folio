import { ref } from 'vue'

const LEFT_KEY = 'folio.reading.leftCollapsed'
const RIGHT_KEY = 'folio.reading.rightCollapsed'

function readFlag(key) {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

function writeFlag(key, value) {
  try {
    localStorage.setItem(key, value ? '1' : '0')
  } catch {
    /* 隐私模式可能不可用 */
  }
}

export function useReadingLayout() {
  const leftCollapsed = ref(readFlag(LEFT_KEY))
  const rightCollapsed = ref(readFlag(RIGHT_KEY))

  function toggleLeft() {
    leftCollapsed.value = !leftCollapsed.value
    writeFlag(LEFT_KEY, leftCollapsed.value)
  }

  function toggleRight() {
    rightCollapsed.value = !rightCollapsed.value
    writeFlag(RIGHT_KEY, rightCollapsed.value)
  }

  return { leftCollapsed, rightCollapsed, toggleLeft, toggleRight }
}
