<script setup lang="ts">
/**
 * VirtualGrid - 虚拟网格组件
 *
 * 用于固定尺寸项目的虚拟滚动网格布局
 * 适合文件夹页面等场景
 */
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'

export interface GridItem {
  id: string
  [key: string]: unknown
}

const props = withDefaults(
  defineProps<{
    items: GridItem[]
    itemWidth?: number
    itemHeight?: number
    gap?: number
    buffer?: number
  }>(),
  {
    itemWidth: 230,
    itemHeight: 280,
    gap: 15,
    buffer: 300,
  }
)

const emit = defineEmits<{
  (e: 'scroll-end'): void
}>()

defineExpose({
  scrollToTop,
  scrollToItem,
  refresh,
})

const containerRef = ref<HTMLElement>()
const scrollTop = ref(0)
const containerHeight = ref(0)
const containerWidth = ref(0)

/* 计算每行项目数 */
const columnsPerRow = computed(() => {
  if (containerWidth.value === 0) return 1
  return Math.max(1, Math.floor((containerWidth.value + props.gap) / (props.itemWidth + props.gap)))
})

/* 计算行高 */
const rowHeight = computed(() => props.itemHeight + props.gap)

/* 计算总行数 */
const totalRows = computed(() => Math.ceil(props.items.length / columnsPerRow.value))

/* 计算总高度 */
const totalHeight = computed(() => totalRows.value * rowHeight.value - props.gap)

/* 计算可见行范围 */
const visibleRange = computed(() => {
  const startRow = Math.max(0, Math.floor((scrollTop.value - props.buffer) / rowHeight.value))
  const endRow = Math.min(
    totalRows.value - 1,
    Math.ceil((scrollTop.value + containerHeight.value + props.buffer) / rowHeight.value)
  )
  return { startRow, endRow }
})

/* 计算可见项 */
const visibleItems = computed(() => {
  const { startRow, endRow } = visibleRange.value
  const result: Array<{ item: GridItem; index: number; style: Record<string, string> }> = []

  for (let row = startRow; row <= endRow; row++) {
    for (let col = 0; col < columnsPerRow.value; col++) {
      const index = row * columnsPerRow.value + col
      if (index >= props.items.length) break

      result.push({
        item: props.items[index],
        index,
        style: {
          position: 'absolute',
          top: `${row * rowHeight.value}px`,
          left: `${col * (props.itemWidth + props.gap)}px`,
          width: `${props.itemWidth}px`,
          height: `${props.itemHeight}px`,
        },
      })
    }
  }

  return result
})

/* 滚动处理 */
let rafId: number | null = null
const handleScroll = () => {
  if (rafId) return

  rafId = requestAnimationFrame(() => {
    if (containerRef.value) {
      scrollTop.value = containerRef.value.scrollTop

      const scrollHeight = containerRef.value.scrollHeight
      const clientHeight = containerRef.value.clientHeight
      const currentScroll = containerRef.value.scrollTop

      if (scrollHeight - currentScroll - clientHeight < 100) {
        emit('scroll-end')
      }
    }
    rafId = null
  })
}

/* 容器尺寸观察 */
let resizeObserver: ResizeObserver | null = null

const updateContainerSize = () => {
  if (containerRef.value) {
    containerWidth.value = containerRef.value.clientWidth
    containerHeight.value = containerRef.value.clientHeight
  }
}

function scrollToTop() {
  containerRef.value?.scrollTo({ top: 0, behavior: 'smooth' })
}

function scrollToItem(index: number) {
  const row = Math.floor(index / columnsPerRow.value)
  const top = row * rowHeight.value
  containerRef.value?.scrollTo({ top, behavior: 'smooth' })
}

function refresh() {
  updateContainerSize()
  nextTick(() => {
    handleScroll()
  })
}

/* 图片懒加载 */
const imageObserver = ref<IntersectionObserver | null>(null)
const loadedImages = ref<Set<string>>(new Set())

const initImageObserver = () => {
  imageObserver.value = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          const img = entry.target as HTMLImageElement
          const itemId = img.dataset.itemId
          const src = img.dataset.src

          if (src && itemId && !loadedImages.value.has(itemId)) {
            img.src = src
            loadedImages.value.add(itemId)
            imageObserver.value?.unobserve(img)
          }
        }
      })
    },
    {
      root: containerRef.value,
      rootMargin: `${props.buffer}px 0px`,
      threshold: 0.01,
    }
  )
}

const observeImage = (el: HTMLImageElement | null) => {
  if (el && imageObserver.value) {
    imageObserver.value.observe(el)
  }
}

watch(
  () => props.items,
  (newItems) => {
    const newIds = new Set(newItems.map((i) => i.id))
    loadedImages.value.forEach((id) => {
      if (!newIds.has(id)) {
        loadedImages.value.delete(id)
      }
    })
  },
  { deep: false }
)

onMounted(() => {
  updateContainerSize()
  initImageObserver()

  if (containerRef.value) {
    containerRef.value.addEventListener('scroll', handleScroll, { passive: true })

    resizeObserver = new ResizeObserver(() => {
      updateContainerSize()
    })
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  if (rafId) {
    cancelAnimationFrame(rafId)
  }

  if (containerRef.value) {
    containerRef.value.removeEventListener('scroll', handleScroll)
  }

  if (resizeObserver) {
    resizeObserver.disconnect()
  }

  if (imageObserver.value) {
    imageObserver.value.disconnect()
  }
})
</script>

<template>
  <div ref="containerRef" class="virtual-grid-container">
    <div class="virtual-grid-content" :style="{ height: `${totalHeight}px` }">
      <div v-for="{ item, index, style } in visibleItems" :key="item.id" class="virtual-grid-item" :style="style">
        <slot :item="item" :index="index" :observe-image="observeImage" :is-loaded="loadedImages.has(item.id)">
          <div class="default-item">{{ item.id }}</div>
        </slot>
      </div>
    </div>
  </div>
</template>

<style scoped>
.virtual-grid-container {
  width: 100%;
  height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
  position: relative;
}

.virtual-grid-content {
  position: relative;
  width: 100%;
}

.virtual-grid-item {
  will-change: transform;
  contain: layout style paint;
}

.default-item {
  width: 100%;
  height: 100%;
  background: var(--color-background-700);
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
