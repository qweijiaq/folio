<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  modelValue: { default: null },
  options: { type: Array, default: () => [] },
  id: { type: String, default: '' },
  placeholder: { type: String, default: '请选择' },
  disabled: { type: Boolean, default: false },
  ariaLabel: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue', 'change'])

const open = ref(false)
const active = ref(-1)
const trigger = ref(null)
const menu = ref(null)
const menuStyle = ref({})

const normalized = computed(() =>
  (props.options || []).map((option) => {
    if (option && typeof option === 'object' && 'value' in option) {
      return {
        value: option.value,
        label: String(option.label ?? option.name ?? option.value ?? ''),
        description: option.description || '',
        disabled: Boolean(option.disabled)
      }
    }
    return { value: option, label: String(option ?? ''), description: '', disabled: false }
  })
)

function sameValue(a, b) {
  if (a === b) return true
  if (a === null || a === undefined || b === null || b === undefined) return false
  return String(a) === String(b)
}

const selectedIndex = computed(() => normalized.value.findIndex((option) => sameValue(option.value, props.modelValue)))
const selected = computed(() => (selectedIndex.value >= 0 ? normalized.value[selectedIndex.value] : null))
const displayLabel = computed(() => selected.value?.label || props.placeholder)

function positionMenu() {
  if (!trigger.value) return
  const rect = trigger.value.getBoundingClientRect()
  const width = Math.max(rect.width, 160)
  const left = Math.max(8, Math.min(rect.left, window.innerWidth - width - 8))
  const below = window.innerHeight - rect.bottom
  const above = rect.top
  let top = rect.bottom + 6
  let maxHeight = Math.min(280, below - 12)
  if (maxHeight < 120 && above > below) {
    maxHeight = Math.min(280, above - 12)
    top = rect.top - maxHeight - 6
  }
  menuStyle.value = {
    position: 'fixed',
    left: `${left}px`,
    top: `${top}px`,
    width: `${width}px`,
    maxHeight: `${Math.max(80, maxHeight)}px`
  }
}

function openMenu() {
  if (props.disabled || open.value) return
  open.value = true
  active.value = selectedIndex.value >= 0 ? selectedIndex.value : 0
  nextTick(() => {
    positionMenu()
    scrollActive()
  })
  document.addEventListener('mousedown', onDocumentMouseDown)
  window.addEventListener('resize', closeMenu)
  window.addEventListener('scroll', onWindowScroll, true)
}

function closeMenu() {
  open.value = false
  document.removeEventListener('mousedown', onDocumentMouseDown)
  window.removeEventListener('resize', closeMenu)
  window.removeEventListener('scroll', onWindowScroll, true)
}

function toggle() {
  if (open.value) closeMenu()
  else openMenu()
}

function choose(option) {
  if (!option || option.disabled) return
  emit('update:modelValue', option.value)
  emit('change', option.value)
  closeMenu()
  nextTick(() => trigger.value?.focus())
}

function moveActive(delta) {
  if (!open.value) {
    openMenu()
    return
  }
  const total = normalized.value.length
  if (!total) return
  let next = active.value
  for (let i = 0; i < total; i += 1) {
    next = (next + delta + total) % total
    if (!normalized.value[next].disabled) break
  }
  active.value = next
  nextTick(scrollActive)
}

function scrollActive() {
  const options = menu.value?.querySelectorAll('.select-box-option')
  options?.[active.value]?.scrollIntoView({ block: 'nearest' })
}

function onKeydown(event) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    moveActive(1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    moveActive(-1)
  } else if (event.key === 'Home') {
    event.preventDefault()
    active.value = 0
    scrollActive()
  } else if (event.key === 'End') {
    event.preventDefault()
    active.value = Math.max(0, normalized.value.length - 1)
    scrollActive()
  } else if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    if (!open.value) openMenu()
    else choose(normalized.value[active.value])
  } else if (event.key === 'Escape') {
    if (open.value) {
      event.preventDefault()
      closeMenu()
    }
  } else if (event.key === 'Tab') {
    closeMenu()
  }
}

function onDocumentMouseDown(event) {
  if (trigger.value?.contains(event.target) || menu.value?.contains(event.target)) return
  closeMenu()
}

function onWindowScroll(event) {
  if (menu.value && (event.target === menu.value || menu.value.contains(event.target))) return
  closeMenu()
}

watch(
  () => props.disabled,
  (value) => {
    if (value) closeMenu()
  }
)

onMounted(() => {
  if (props.disabled) closeMenu()
})

onBeforeUnmount(closeMenu)
</script>

<template>
  <div class="select-box" :class="{ open, disabled }">
    <button
      :id="id"
      ref="trigger"
      class="select-box-trigger"
      type="button"
      :disabled="disabled"
      :aria-label="ariaLabel || undefined"
      aria-haspopup="listbox"
      :aria-expanded="open"
      @click="toggle"
      @keydown="onKeydown"
    >
      <span class="select-box-label" :class="{ placeholder: !selected }">{{ displayLabel }}</span>
      <Icon name="chevronDown" :size="15" class="select-box-arrow" />
    </button>

    <Teleport to="body">
      <Transition name="dropdown">
        <div
          v-if="open"
          ref="menu"
          class="select-box-menu"
          :style="menuStyle"
          role="listbox"
          :aria-label="ariaLabel || undefined"
        >
          <button
            v-for="(option, index) in normalized"
            :key="`${index}-${option.label}`"
            class="select-box-option"
            :class="{ active: index === active, selected: index === selectedIndex, disabled: option.disabled }"
            type="button"
            role="option"
            :aria-selected="index === selectedIndex"
            :disabled="option.disabled"
            @mouseenter="active = index"
            @click="choose(option)"
          >
            <span class="select-box-option-main">
              <span class="select-box-option-label">{{ option.label }}</span>
              <small v-if="option.description">{{ option.description }}</small>
            </span>
            <Icon v-if="index === selectedIndex" name="check" :size="14" />
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
