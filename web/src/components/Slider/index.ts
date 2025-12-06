import type { App } from 'vue'
import Slider from './index.vue'
import type { SliderEmits, SliderProps } from './types'

export { Slider }
export type { SliderProps, SliderEmits }

// 默认导出组件本身，以支持懒加载
export default Slider

// 插件安装方法
export const CyberSliderPlugin = {
  install(app: App) {
    app.component('CyberSlider', Slider)
  },
}
