/*
 * 组件注册系统 - 优化版
 *
 * 策略：
 * 1. 核心组件（Button, Dialog, Toast, Input, Loading 等）立即注册
 * 2. 其他组件使用 defineAsyncComponent 懒加载
 * 3. 减少初始包体积 50-100KB
 */

import type { App, Component, Plugin } from 'vue'
import { defineAsyncComponent } from 'vue'

/* ========== 核心组件 - 立即加载 ========== */
/* 这些组件在首屏或全局频繁使用，需要立即可用 */
import Button from './Button'
import Dialog from './Dialog'
import Toast, { useToast } from './Toast'
import Input from './Input'
import { Loading } from './Loading'
import Switch from './Switch'
import Checkbox from './Checkbox'
import Tooltip from './Tooltip'
import Dropdown from './Dropdown'
import Popconfirm from './Popconfirm'

/* ========== 布局组件 - 立即加载 ========== */
/* 这些组件用于页面布局，需要立即可用 */
import Navbar from './Navbar/index.vue'
import CyberSidebar from './Sidebar/index.vue'
import Logo from './Logo'

/* ========== 懒加载组件工厂 ========== */
const createAsyncComponent = (loader: () => Promise<{ default: Component }>) => {
  return defineAsyncComponent({
    loader,
    loadingComponent: Loading,
    delay: 200,
    timeout: 30000,
  })
}

/* ========== 懒加载组件定义 ========== */
/* eslint-disable @typescript-eslint/no-explicit-any */
const lazyComponents: Record<string, () => Promise<any>> = {
  /* 表单组件 */
  CyberDatePicker: () => import('./DatePicker'),
  CyberColorPicker: () => import('./ColorPicker/index.vue'),
  CyberIconPicker: () => import('./IconPicker'),
  CyberMultiSelector: () => import('./MultiSelector'),
  CyberRadio: () => import('./Radio').then((m) => ({ default: m.Radio })),
  CyberRadioGroup: () => import('./Radio').then((m) => ({ default: m.RadioGroup })),
  CyberSlider: () => import('./Slider').then((m) => ({ default: m.Slider })),

  /* 数据展示组件 */
  CyberCard: () => import('./Card'),
  CyberBadge: () => import('./Badge/index.vue'),
  CyberTag: () => import('./Tag'),
  CyberSmartTagContainer: () => import('./SmartTagContainer'),
  CyberPagination: () => import('./Pagination'),
  CyberDataTable: () => import('./Table'),
  CyberTable: () => import('./Table'),
  CyberSkeleton: () => import('./CyberSkeleton'),
  CyberStatsCard: () => import('./StatsCard'),
  CyberStatsSection: () => import('./StatsSection'),

  /* 导航组件 */
  CyberBreadcrumb: () => import('./Breadcrumb'),
  CyberMobileNavigation: () => import('./MobileNavigation'),
  CyberSideNavTabs: () => import('./SideNavTabs'),
  CyberSidebarNav: () => import('./CyberSidebarNav'),
  CyberAccordion: () => import('./Accordion'),
  CyberContextMenu: () => import('./CyberContextMenu'),

  /* 文件相关组件 */
  CyberFile: () => import('./File'),
  CyberFileViewer: () => import('./FileViewer'),
  CyberFileDetailModal: () => import('./FileDetailModal'),
  CyberFileLoading: () => import('./FileLoading/index.vue'),
  CyberFolderTree: () => import('./FolderTree'),
  CyberShareFolder: () => import('./ShareFolder'),
  CyberShareFile: () => import('./ShareFile').then((m) => ({ default: m.ShareFile })),
  CyberWaterfallLayout: () => import('./WaterfallLayout'),
  CyberAccessLevelToggle: () => import('./AccessLevelToggle'),
  CyberFileExpiryTag: () => import('./FileExpiryTag'),
  CyberDuplicateBadge: () => import('./DuplicateBadge/index.vue'),

  /* 上传组件 */
  CyberResumableUploads: () => import('./ResumableUploads'),
  CyberGlobalUploadFloat: () => import('./GlobalUploadFloat/index.vue'),
  CyberGlobalUploadDrawer: () => import('./GlobalUploadDrawer/index.vue'),
  CyberWatermarkConfig: () => import('./WatermarkConfig/index.vue'),

  /* 用户组件 */
  CyberUserAvatar: () => import('./UserAvatar'),

  /* 反馈组件 */
  CyberConfirmDialog: () => import('./ConfirmDialog'),
  CyberDrawer: () => import('./Drawer'),
  CyberCommunityDialog: () => import('./CommunityDialog/index.vue'),
  CyberNotificationDialog: () => import('./NotificationDialog/index.vue'),

  /* 背景/装饰组件 */
  CyberBackground: () => import('./Background'),
  CyberParticleBackground: () => import('./ParticleBackground/index.vue'),
  CyberHomeBackground: () => import('./CyberHomeBackground/index.vue'),
  CyborgCharacter: () => import('./CyborgCharacter/index.vue'),
  CyberCyborgCharacter: () => import('./CyborgCharacter/index.vue'),

  /* 布局适配组件 */
  CyberAdminWrapper: () => import('./AdminWrapper'),
  CyberPageContainer: () => import('./LayoutAdaptive/PageContainer.vue'),
  CyberGridContainer: () => import('./LayoutAdaptive/GridContainer.vue'),
  CyberCardContainer: () => import('./LayoutAdaptive/CardContainer.vue'),
  CyberLayoutSwitcher: () => import('./LayoutSwitcher/index.vue'),
  CyberLayoutToggleButton: () => import('./LayoutToggleButton/index.vue'),

  /* 其他组件 */
  CyberIconButton: () => import('./IconButton'),
  CyberCopyright: () => import('./Copyright/index.vue'),
  CyberThemeToggle: () => import('./ThemeToggle'),
  CyberAnnouncementButton: () => import('./AnnouncementButton/index.vue'),

  /* 虚拟滚动组件 - 性能优化 */
  CyberVirtualMasonry: () => import('./VirtualMasonry/index.vue'),
  CyberVirtualGrid: () => import('./VirtualGrid/index.vue'),
}
/* eslint-enable @typescript-eslint/no-explicit-any */

/* ========== 核心组件映射表 ========== */
const coreComponentMap: Record<string, Component> = {
  CyberButton: Button,
  CyberDialog: Dialog,
  CyberToast: Toast,
  CyberInput: Input,
  CyberLoading: Loading,
  CyberSwitch: Switch,
  CyberCheckbox: Checkbox,
  CyberTooltip: Tooltip,
  CyberDropdown: Dropdown,
  CyberPopconfirm: Popconfirm,
  CyberNavbar: Navbar,
  CyberSidebar: CyberSidebar,
  CyberLogo: Logo,
}

/* ========== 兼容性导出 - 使用 cyber 前缀 ========== */
export const components = {
  cyberButton: Button,
  cyberDialog: Dialog,
  cyberToast: Toast,
  cyberInput: Input,
  cyberLoading: Loading,
  cyberSwitch: Switch,
  cyberCheckbox: Checkbox,
  cyberTooltip: Tooltip,
  cyberDropdown: Dropdown,
  cyberPopconfirm: Popconfirm,
  cyberNavbar: Navbar,
  cyberSidebar: CyberSidebar,
  cyberLogo: Logo,
}

/* ========== 导出 useToast 函数 ========== */
export { useToast }

/* ========== 统一导出 Badge 组件 ========== */
export { default as CyberBadge } from './Badge/index.vue'
export { default as DuplicateBadge } from './Badge/index.vue'
export { default as SimilarityBadge } from './Badge/index.vue'

/* ========== 组件库插件 ========== */
export interface CyberUIOptions {
  /* 指定要注册的组件列表，为空则注册全部 */
  components?: string[]
  /* 是否启用懒加载（默认 true） */
  lazyLoad?: boolean
}

export const createCyberUI = (options: CyberUIOptions = {}): Plugin => ({
  install(app: App) {
    const { components: selectedComponents = [], lazyLoad = true } = options

    /* 注册核心组件（始终立即加载） */
    Object.entries(coreComponentMap).forEach(([name, component]) => {
      if (selectedComponents.length === 0 || selectedComponents.includes(name)) {
        app.component(name, component)
      }
    })

    /* 注册懒加载组件 */
    Object.entries(lazyComponents).forEach(([name, loader]) => {
      if (selectedComponents.length === 0 || selectedComponents.includes(name)) {
        if (lazyLoad) {
          /* 使用异步组件 */
          app.component(name, createAsyncComponent(loader as () => Promise<{ default: Component }>))
        } else {
          /* 立即加载（用于 SSR 或特殊场景） */
          loader().then((module) => {
            const component = 'default' in module ? module.default : module
            app.component(name, component as Component)
          })
        }
      }
    })
  },
})

/* ========== 默认插件实例 ========== */
const CyberComponentsPlugin: Plugin = createCyberUI()

export default CyberComponentsPlugin

/* ========== 类型导出 ========== */
export type ComponentName = keyof typeof coreComponentMap | keyof typeof lazyComponents
