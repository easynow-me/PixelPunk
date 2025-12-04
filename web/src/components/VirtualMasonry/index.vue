<script setup lang="ts">
/**
 * VirtualMasonry - 虚拟瀑布流组件
 *
 * 核心优化策略:
 * 1. 只渲染可视区域内的图片项
 * 2. 使用 Intersection Observer 实现图片懒加载
 * 3. 回收离开视口的 DOM 元素
 * 4. 预渲染上下缓冲区以提升滚动体验
 */
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'

export interface MasonryItem {
  id: string
  width: number
  height: number
  [key: string]: unknown
}

interface PositionedItem {
  item: MasonryItem
  top: number
  left: number
  width: number
  height: number
  column: number
}

const props = withDefaults(
  defineProps<{
    items: MasonryItem[]
    columnCount?: number
    gap?: number
    buffer?: number // 缓冲区高度（像素）
    minColumnWidth?: number
    estimatedItemHeight?: number
  }>(),
  {
    columnCount: 0, // 0 表示自动计算
    gap: 16,
    buffer: 500,
    minColumnWidth: 200,
    estimatedItemHeight: 250,
  }
)

const emit = defineEmits<{
  (e: 'scroll-end'): void
  (e: 'item-visible', item: MasonryItem): void
}>()

defineExpose({
  scrollToTop,
  refresh,
})

const containerRef = ref<HTMLElement>()
const scrollTop = ref(0)
const containerHeight = ref(0)
const containerWidth = ref(0)

/* 动态计算列数 */
const actualColumnCount = computed(() => {
  if (props.columnCount > 0) return props.columnCount
  if (containerWidth.value === 0) return 4
  return Math.max(2, Math.floor(containerWidth.value / props.minColumnWidth))
})

/* 计算每列宽度 */
const columnWidth = computed(() => {
  const totalGap = (actualColumnCount.value - 1) * props.gap
  return (containerWidth.value - totalGap) / actualColumnCount.value
})

/* 计算所有项的位置 */
const positionedItems = computed<PositionedItem[]>(() => {
  if (!props.items.length || columnWidth.value <= 0) return []

  const columnHeights = new Array(actualColumnCount.value).fill(0)
  const positions: PositionedItem[] = []

  for (const item of props.items) {
    /* 找到最短的列 */
    const minHeight = Math.min(...columnHeights)
    const columnIndex = columnHeights.indexOf(minHeight)

    /* 计算项目高度（保持宽高比） */
    const aspectRatio = (item.height || 100) / (item.width || 100)
    const itemHeight = columnWidth.value * aspectRatio

    const position: PositionedItem = {
      item,
      top: minHeight,
      left: columnIndex * (columnWidth.value + props.gap),
      width: columnWidth.value,
      height: itemHeight,
      column: columnIndex,
    }

    positions.push(position)
    columnHeights[columnIndex] = minHeight + itemHeight + props.gap
  }

  return positions
})

/* 计算总高度 */
const totalHeight = computed(() => {
  if (!positionedItems.value.length) return 0
  return Math.max(...positionedItems.value.map((p) => p.top + p.height)) + props.gap
})

/* 计算可见项（带缓冲区） */
const visibleItems = computed(() => {
  const viewTop = scrollTop.value - props.buffer
  const viewBottom = scrollTop.value + containerHeight.value + props.buffer

  return positionedItems.value.filter((pos) => {
    const itemTop = pos.top
    const itemBottom = pos.top + pos.height
    return itemBottom >= viewTop && itemTop <= viewBottom
  })
})

/* 滚动处理（使用 RAF 节流） */
let rafId: number | null = null
const handleScroll = () => {
  if (rafId) return

  rafId = requestAnimationFrame(() => {
    if (containerRef.value) {
      scrollTop.value = containerRef.value.scrollTop

      /* 检测是否滚动到底部 */
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

/* 滚动到顶部 */
function scrollToTop() {
  containerRef.value?.scrollTo({ top: 0, behavior: 'smooth' })
}

/* 刷新布局 */
function refresh() {
  updateContainerSize()
  nextTick(() => {
    handleScroll()
  })
}

/* 图片懒加载观察器 */
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

            /* 触发可见事件 */
            const item = props.items.find((i) => i.id === itemId)
            if (item) {
              emit('item-visible', item)
            }
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

/* 注册图片到观察器 */
const observeImage = (el: HTMLImageElement | null) => {
  if (el && imageObserver.value) {
    imageObserver.value.observe(el)
  }
}

/* 监听 items 变化，清理已移除项的加载状态 */
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
  <div ref="containerRef" class="virtual-masonry-container">
    <div class="virtual-masonry-content" :style="{ height: `${totalHeight}px` }">
      <div
        v-for="pos in visibleItems"
        :key="pos.item.id"
        class="virtual-masonry-item"
        :style="{
          position: 'absolute',
          top: `${pos.top}px`,
          left: `${pos.left}px`,
          width: `${pos.width}px`,
          height: `${pos.height}px`,
        }"
      >
        <slot :item="pos.item" :observe-image="observeImage" :is-loaded="loadedImages.has(pos.item.id)">
          <!-- 默认渲染插槽 -->
          <div class="default-item">{{ pos.item.id }}</div>
        </slot>
      </div>
    </div>
  </div>
</template>

<style scoped>
.virtual-masonry-container {
  width: 100%;
  height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
  position: relative;
}

.virtual-masonry-content {
  position: relative;
  width: 100%;
}

.virtual-masonry-item {
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
