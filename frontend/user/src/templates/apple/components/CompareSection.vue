<template>
  <div class="cmp" :style="{ '--cmp-cols': columns }">
    <div class="cmp-scroll">
      <div class="cmp-grid">
        <!-- 表头：每个商品一列 -->
        <div v-for="item in data.items" :key="`h-${item.id}`" class="cmp-head">
          <RouterLink :to="`/products/${item.slug}`" class="cmp-head-link">
            <img v-if="item.image" class="cmp-img" :src="getImageUrl(item.image)" :alt="getLocalizedText(item.title)" />
            <span class="cmp-badge">{{ hasText(item.badge) ? getLocalizedText(item.badge) : '' }}</span>
            <span class="cmp-name">{{ getLocalizedText(item.title) }}</span>
            <span v-if="hasText(item.tagline)" class="cmp-tagline">{{ getLocalizedText(item.tagline) }}</span>
            <span v-if="data.show_price" class="cmp-price">{{ priceLabel(item) }}</span>
          </RouterLink>
          <button v-if="removable" type="button" class="cmp-remove" @click="emit('remove', item.id)">
            {{ t('comparePage.remove') }}
          </button>
        </div>

        <!-- 每个维度一段：图标 / 主值 / 说明 -->
        <template v-for="dim in data.dimensions" :key="dim.key">
          <div v-for="item in data.items" :key="`${dim.key}-${item.id}`" class="cmp-cell">
            <span v-if="dim.icon" class="cmp-icon">
              <span v-if="isSymbolGlyph(dim.icon)" class="cmp-glyph">{{ dim.icon }}</span>
              <span v-else-if="isRawSvgIcon(dim.icon)" v-html="rawSvgIcon(dim.icon)" />
              <component v-else-if="iconComponent(dim.icon)" :is="iconComponent(dim.icon)" />
            </span>
            <span class="cmp-value" :aria-label="getLocalizedText(dim.label)">{{ mainValue(item, dim) }}</span>
            <span v-if="detailLines(item, dim).length" class="cmp-detail">
              <span v-for="(line, i) in detailLines(item, dim)" :key="i">{{ line }}</span>
            </span>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Aperture, BatteryCharging, Bluetooth, Box, Camera, ChevronRight, CircleDot, Cpu, Download,
  Droplets, Fingerprint, Gamepad2, Gauge, HardDrive, Headphones, Keyboard, Laptop, Layers, Lock,
  Maximize, MemoryStick, Monitor, Moon, Mouse, Package, Palette, Plug, Radio, Ruler, ShieldCheck,
  Smartphone, Speaker, Sun, Tablet, Thermometer, Timer, Truck, Upload, Usb, Video, Volume2, Watch, Wifi, Zap,
} from 'lucide-vue-next'
import { useLocalized } from '../../../composables/useProduct'
import { getImageUrl } from '../../../utils/image'

const props = withDefaults(defineProps<{
  data: any
  removable?: boolean
  /** 价格文案后缀，苹果是「RMB 15,999 起」 */
  priceSuffix?: string
}>(), { removable: false, priceSuffix: '' })

const emit = defineEmits<{ remove: [number] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()

const columns = computed(() => Math.min(5, Math.max(2, props.data?.items?.length || 2)))

const hasText = (value: any) => Boolean(value && getLocalizedText(value))

const priceLabel = (item: any) => {
  const price = formatPrice(item.price_amount, siteCurrency.value)
  return props.priceSuffix ? `${price} ${props.priceSuffix}` : price
}

const mainValue = (item: any, dim: any) => {
  const raw = item?.values?.[dim.key]
  const text = typeof raw === 'string' ? raw : getLocalizedText(raw || {})
  if (!text) return '—'
  if (!dim.unit || text.trim().endsWith(dim.unit)) return text
  return `${text}${dim.unit}`
}

const detailLines = (item: any, dim: any): string[] => {
  const raw = item?.details?.[dim.key]
  const text = typeof raw === 'string' ? raw : getLocalizedText(raw || {})
  if (!text) return []
  return String(text)
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
}

/* ---------- 图标 ----------
   后台填的是 SF Symbols 字符（非 ASCII），用苹果字体按官网的字重/大小渲染；
   也兼容旧的 lucide 名字与自定义 SVG。 */
const isSymbolGlyph = (name: string) => Boolean(name) && /[^\x00-\x7F]/.test(String(name)) && !String(name).startsWith('svg:')
const isRawSvgIcon = (name: string) => String(name || '').startsWith('svg:')
const rawSvgIcon = (name: string) => String(name || '').slice(4)
const ICON_MAP: Record<string, any> = {
  iphone: Smartphone, ipad: Tablet, laptopcomputer: Laptop, display: Monitor, cpu: Cpu,
  memorychip: MemoryStick, camera: Camera, video: Video, apature: Aperture, bolt: Zap,
  battery: BatteryCharging, wifi: Wifi, bluetooth: Bluetooth, speaker: Speaker,
  headphones: Headphones, volume: Volume2, applewatch: Watch, keyboard: Keyboard, mouse: Mouse,
  powerplug: Plug, 'cable.connector': Usb, internaldrive: HardDrive, icloud: Box,
  'arrow.down': Download, 'arrow.up': Upload, 'checkmark.shield': ShieldCheck, lock: Lock,
  sun: Sun, moon: Moon, paintpalette: Palette, 'square.stack': Layers, shippingbox: Package,
  truck: Truck, timer: Timer, gauge: Gauge, thermometer: Thermometer, drop: Droplets, ruler: Ruler,
  'arrow.up.left.and.arrow.down.right': Maximize, 'dot.radiowaves': Radio, gamecontroller: Gamepad2,
  touchid: Fingerprint, circle: CircleDot, 'chevron.right': ChevronRight,
}
const iconComponent = (name: string) => {
  const key = String(name || '').toLowerCase()
  return ICON_MAP[key] || Object.entries(ICON_MAP).find(([k]) => key.includes(k))?.[1] || null
}
</script>

<style scoped>
/* 对齐 apple.com.cn 购买页「哪款适合你」模块：
   列宽 200、无左侧标签列、图标 56、主值 28/600、说明 12、价格 14、名称 28/600 */
.cmp {
  --cmp-col: 200px;
}
.cmp-scroll {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.cmp-grid {
  display: grid;
  grid-template-columns: repeat(var(--cmp-cols), minmax(var(--cmp-col), 1fr));
  column-gap: 8px;
  min-width: calc(var(--cmp-cols) * var(--cmp-col));
}

/* 表头 */
.cmp-head {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-bottom: 22px;
  border-bottom: 1px solid var(--st-hairline, #d2d2d7);
  text-align: center;
}
.cmp-head-link {
  display: flex;
  flex-direction: column;
  align-items: center;
  color: inherit;
  text-decoration: none;
}
.cmp-img {
  width: 196px;
  height: 168px;
  object-fit: contain;
  margin-bottom: 12px;
}
.cmp-badge {
  min-height: 16px;
  margin-bottom: 4px;
  font-size: 12px;
  line-height: 1.3;
  color: var(--st-orange, #b64400);
}
.cmp-name {
  font-size: 28px;
  font-weight: 600;
  line-height: 35px;
  letter-spacing: 0.196px;
  color: var(--st-text);
}
.cmp-tagline {
  display: block;
  min-height: 69px;
  margin-top: 8px;
  max-width: 190px;
  overflow: hidden;
  font-size: 17px;
  line-height: 23px;
  color: var(--st-text);
}
.cmp-price {
  margin-top: 14px;
  font-size: 14px;
  line-height: 19px;
  color: var(--st-text);
}
.cmp-remove {
  margin-top: 10px;
  border: 0;
  background: none;
  font-size: 13px;
  color: var(--st-link, #06c);
  cursor: pointer;
}
.cmp-remove:hover {
  text-decoration: underline;
}

/* 数据格 */
.cmp-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 56px 6px 0;
  text-align: center;
}
.cmp-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  margin-bottom: 14px;
  color: var(--st-text);
}
.cmp-icon :deep(svg) {
  width: 56px;
  height: 56px;
  stroke-width: 1.1;
}
/* SF Symbols 字形：与苹果对比页一致 —— 56px / Light / 纯黑 */
.cmp-glyph {
  font-family: 'SF Pro', 'SF Pro Display', 'SF Pro SC', -apple-system, 'Apple Symbols', sans-serif;
  font-size: 56px;
  font-weight: 300;
  line-height: 1;
  letter-spacing: 0;
  color: var(--st-text, #1d1d1f);
}
.cmp-value {
  font-size: 28px;
  font-weight: 600;
  line-height: 35px;
  letter-spacing: 0.196px;
  color: var(--st-text);
  white-space: pre-line;
}
.cmp-detail {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin-top: 10px;
  font-size: 12px;
  line-height: 16px;
  color: var(--st-text-2);
  white-space: pre-line;
}

@media (max-width: 734px) {
  .cmp {
    --cmp-col: 172px;
  }
  .cmp-img {
    width: 160px;
    height: 140px;
  }
  .cmp-name,
  .cmp-value {
    font-size: 21px;
    line-height: 26px;
  }
  .cmp-tagline {
    font-size: 14px;
    line-height: 19px;
  }
  .cmp-cell {
    padding-top: 40px;
  }
  .cmp-icon,
  .cmp-icon :deep(svg) {
    width: 40px;
    height: 40px;
  }
  .cmp-glyph {
    font-size: 40px;
  }
}
</style>
