<script setup lang="ts">
/**
 * GalleryMasonryVirtual - 虚拟瀑布流画廊组件
 *
 * 性能优化版本:
 * - 虚拟滚动：只渲染可视区域内的项目
 * - 图片懒加载：使用 Intersection Observer
 * - 内存优化：回收离开视口的 DOM 元素
 */
import { computed, ref } from 'vue'
import type { FileInfo } from '@/api/common'
import { useRouter } from 'vue-router'
import { useTexts } from '@/composables/useTexts'
import VirtualMasonry from '@/components/VirtualMasonry/index.vue'

const { $t } = useTexts()

const props = defineProps<{
  files: FileInfo[]
  selectMode?: boolean
  isFileSelected?: (file: FileInfo) => boolean
  isVectorSearch?: boolean
  columnCount?: number
  showTags?: boolean
}>()

const emit = defineEmits<{
  (e: 'view', id: string): void
  (e: 'download', file: FileInfo): void
  (e: 'details', id: string): void
  (e: 'select', file: FileInfo): void
  (e: 'load-more'): void
}>()

const router = useRouter()
const virtualMasonryRef = ref<InstanceType<typeof VirtualMasonry>>()

/* 转换文件为 MasonryItem 格式 */
const masonryItems = computed(() =>
  props.files.map((file) => ({
    ...file,
    id: file.id,
    width: file.width || 100,
    height: file.height || 100,
  }))
)

/* 动态计算列数 */
const dynamicColumnCount = computed(() => {
  if (props.columnCount) return props.columnCount
  /* 根据视口宽度自动计算，默认返回 4 */
  return 4
})

const handleFileClick = (file: FileInfo) => {
  if (props.selectMode) {
    emit('select', file)
  } else {
    emit('view', file.id)
  }
}

const handleTagClick = (tag: string) => {
  router.push({
    path: '/gallery',
    query: { tags: tag, from: 'gallery' },
  })
}

const openInNewWindow = (file: FileInfo) => {
  const url = file.full_url || file.url
  if (url) {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

const getCurrentDensity = () => {
  const galleryElement = document.querySelector('.gallery-files-fullscreen')
  if (!galleryElement) return 'normal'

  if (galleryElement.classList.contains('density-compact')) return 'compact'
  if (galleryElement.classList.contains('density-comfortable')) return 'comfortable'
  return 'normal'
}

const getMaxVisibleTags = (tags: string[]) => {
  if (!tags || tags.length === 0) return 0

  const currentDensity = getCurrentDensity()
  let maxTags = 2

  if (currentDensity === 'compact') {
    maxTags = 1
  } else if (currentDensity === 'comfortable') {
    maxTags = 2
  }

  return Math.min(maxTags, tags.length)
}

const formatFileSize = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const handleImageLoad = (event: Event) => {
  const img = event.target as HTMLImageElement
  img.classList.add('loaded')
}

const handleImageError = (event: Event) => {
  const img = event.target as HTMLImageElement
  img.src = '/placeholder-file.png'
}

const handleScrollEnd = () => {
  emit('load-more')
}

/* 暴露方法 */
defineExpose({
  scrollToTop: () => virtualMasonryRef.value?.scrollToTop(),
  refresh: () => virtualMasonryRef.value?.refresh(),
})
</script>

<template>
  <div class="gallery-masonry-virtual-container">
    <VirtualMasonry
      ref="virtualMasonryRef"
      :items="masonryItems"
      :column-count="dynamicColumnCount"
      :gap="16"
      :buffer="600"
      :min-column-width="180"
      @scroll-end="handleScrollEnd"
    >
      <template #default="{ item: file, observeImage, isLoaded }">
        <div
          class="masonry-item selectable-item"
          :class="{
            selected: selectMode && isFileSelected?.(file as FileInfo),
            'is-selected': selectMode && isFileSelected?.(file as FileInfo),
            selectable: selectMode,
            'has-tags': showTags && (file as FileInfo).ai_info?.tags?.length,
          }"
          @click="handleFileClick(file as FileInfo)"
        >
          <div
            v-if="selectMode && isFileSelected?.(file as FileInfo)"
            class="selection-indicator file-selected-indicator"
          >
            <i class="fas fa-check" />
          </div>

          <div class="file-container">
            <!-- 懒加载图片 -->
            <img
              :ref="(el) => observeImage(el as HTMLImageElement)"
              :data-src="(file as FileInfo).full_thumb_url || (file as FileInfo).thumb_url"
              :data-item-id="file.id"
              :alt="(file as FileInfo).display_name || (file as FileInfo).original_name"
              class="masonry-file"
              :class="{ loaded: isLoaded }"
              :style="{ aspectRatio: `${file.width}/${file.height}` }"
              @load="handleImageLoad"
              @error="handleImageError"
            />

            <!-- 骨架屏占位 -->
            <div v-if="!isLoaded" class="image-skeleton" />

            <div v-if="!selectMode" class="image-actions">
              <button
                v-if="(file as FileInfo).ai_info"
                class="image-action-icon"
                :title="$t('explore.actions.details')"
                @click.stop="$emit('details', file.id)"
              >
                <i class="fas fa-info-circle" />
              </button>
              <button
                class="image-action-icon"
                :title="$t('explore.actions.preview')"
                @click.stop="openInNewWindow(file as FileInfo)"
              >
                <i class="fas fa-external-link-alt" />
              </button>
              <button
                class="image-action-icon"
                :title="$t('explore.actions.download')"
                @click.stop="$emit('download', file as FileInfo)"
              >
                <i class="fas fa-download" />
              </button>
            </div>

            <div
              v-if="isVectorSearch && (file as FileInfo).similarity !== undefined"
              class="similarity-badge"
              :class="{ 'with-actions': !selectMode }"
            >
              <i class="fas fa-percentage" />
              {{ Math.round(((file as FileInfo).similarity || 0) * 100) }}
            </div>

            <div class="hover-overlay">
              <div class="file-info">
                <h3 class="file-title">{{ (file as FileInfo).display_name || (file as FileInfo).original_name }}</h3>
                <div class="file-meta">
                  <span>{{ file.width }} × {{ file.height }}</span>
                  <span>{{ formatFileSize((file as FileInfo).size) }}</span>
                </div>
              </div>
            </div>
          </div>

          <div v-if="showTags && (file as FileInfo).ai_info?.tags?.length" class="file-tags">
            <CyberTag
              v-for="(tag, idx) in (file as FileInfo).ai_info?.tags?.slice(0, getMaxVisibleTags((file as FileInfo).ai_info?.tags || []))"
              :key="idx"
              variant="primary"
              size="small"
              @click.stop="handleTagClick(tag)"
            >
              {{ tag }}
            </CyberTag>
            <CyberTag
              v-if="((file as FileInfo).ai_info?.tags?.length || 0) > getMaxVisibleTags((file as FileInfo).ai_info?.tags || [])"
              variant="secondary"
              size="small"
            >
              +{{ ((file as FileInfo).ai_info?.tags?.length || 0) - getMaxVisibleTags((file as FileInfo).ai_info?.tags || []) }}
            </CyberTag>
          </div>
        </div>
      </template>
    </VirtualMasonry>
  </div>
</template>

<style scoped>
.gallery-masonry-virtual-container {
  width: 100%;
  height: 100%;
  min-height: 400px;
}

.masonry-item {
  position: relative;
  border-radius: var(--radius-md);
  overflow: hidden;
  background: var(--color-background-700);
  border: 1px solid var(--color-border-default);
  transition: all 0.3s ease;
  cursor: pointer;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.masonry-item:hover {
  box-shadow: var(--shadow-cyber-lg);
  border-color: var(--color-hover-border);
}

.masonry-item.selectable {
  cursor: pointer;
}

.selection-indicator {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 26px;
  height: 26px;
  background: linear-gradient(135deg, var(--color-badge-accent-text), var(--color-brand-500));
  border-radius: var(--radius-full);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
  box-shadow: var(--shadow-glow-md);
  border: 2px solid var(--color-background-700);
  animation: scaleIn var(--transition-fast) var(--ease-out);
}

.selection-indicator i {
  color: var(--color-content-heading);
  font-size: 12px;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
}

.file-container {
  position: relative;
  width: 100%;
  flex: 1;
  overflow: hidden;
  perspective: 1000px;
}

.masonry-file {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
  opacity: 0;
  transition:
    opacity 0.3s ease,
    transform 0.5s cubic-bezier(0.34, 1.56, 0.64, 1);
  transform-style: preserve-3d;
  transform-origin: center center;
}

.masonry-file.loaded {
  opacity: 1;
}

.image-skeleton {
  position: absolute;
  inset: 0;
  background: linear-gradient(
    90deg,
    var(--color-background-700) 0%,
    var(--color-background-600) 50%,
    var(--color-background-700) 100%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

@keyframes shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

.masonry-item:hover .masonry-file {
  transform: scale(1.15) translateZ(30px);
}

.image-actions {
  position: absolute;
  top: 8px;
  left: 8px;
  display: flex;
  gap: 6px;
  z-index: 5;
  opacity: 0;
  transform: translateY(-4px);
  transition: all 0.3s ease;
  pointer-events: none;
}

.masonry-item:hover .image-actions {
  opacity: 1;
  transform: translateY(0);
  pointer-events: auto;
}

.image-action-icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  background: rgba(var(--color-background-900-rgb), 0.9);
  backdrop-filter: blur(12px);
  border: 1.5px solid rgba(var(--color-brand-500-rgb), 0.4);
  color: var(--color-content);
  transition: all 0.2s ease;
  cursor: pointer;
  box-shadow: 0 2px 8px var(--color-overlay-medium);
}

.image-action-icon:hover {
  border-color: var(--color-brand-500);
  color: var(--color-brand-500);
  box-shadow: 0 4px 12px rgba(var(--color-brand-500-rgb), 0.5);
}

.image-action-icon i {
  font-size: 14px;
  transition: all 0.2s ease;
}

.similarity-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  background: linear-gradient(135deg, rgba(var(--color-brand-500-rgb), 0.9), rgba(var(--color-brand-500-rgb), 0.7));
  color: var(--color-text-on-brand);
  padding: 4px 8px;
  border-radius: var(--radius-lg);
  font-size: 11px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 4px;
  backdrop-filter: blur(10px);
  box-shadow: var(--shadow-cyber-sm);
}

.similarity-badge.with-actions {
  top: 8px;
}

.hover-overlay {
  position: absolute;
  inset: 0;
  background: linear-gradient(to bottom, transparent 60%, rgba(0, 0, 0, 0.9) 100%);
  opacity: 0;
  transition: opacity 0.3s ease;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  padding: 0.75rem;
}

.masonry-item:hover .hover-overlay {
  opacity: 1;
}

.file-info {
  color: var(--color-content-heading);
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 0.75rem;
  background: transparent;
  z-index: 1;
}

.file-title {
  font-size: 13px;
  font-weight: 500;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
}

.file-meta {
  display: flex;
  gap: 0.75rem;
  font-size: 11px;
  color: var(--color-content-default);
}

.file-meta span {
  display: flex;
  align-items: center;
  gap: 4px;
}

.file-tags {
  padding: 6px;
  display: flex;
  flex-wrap: nowrap;
  gap: 4px;
  background: rgba(var(--color-background-900-rgb), 0.5);
  backdrop-filter: blur(8px);
  overflow: hidden;
}

@media (max-width: 640px) {
  .image-action-icon {
    width: 28px;
    height: 28px;
  }

  .image-action-icon i {
    font-size: 12px;
  }
}
</style>
