<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">{{ t('admin.homepage.title') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.homepage.subtitle') }}</p>
    </div>

    <!-- 推广单元 -->
    <Card class="p-5">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-base font-semibold">{{ t('admin.homepage.bannersTitle') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.homepage.bannersDesc') }}</p>
        </div>
        <Button size="sm" variant="outline" @click="router.push('/banners')">{{ t('admin.homepage.editBanners') }}</Button>
      </div>

      <div v-if="bannersLoading" class="mt-4 text-sm text-muted-foreground">{{ t('admin.common.loading') }}</div>
      <div v-else class="mt-4 space-y-2">
        <div v-for="banner in banners" :key="banner.id" class="flex flex-wrap items-center gap-3 rounded-lg border border-border p-3">
          <span class="min-w-0 flex-1 truncate text-sm">{{ localized(banner.title) || banner.name }}</span>
          <div class="flex gap-1">
            <button
              v-for="opt in toneOptions"
              :key="opt.value"
              type="button"
              class="h-8 rounded-md border px-3 text-xs"
              :class="(bannerTone(banner) === opt.value) ? 'border-primary bg-primary/10 text-primary' : 'border-border text-muted-foreground'"
              @click="setBannerTone(banner, opt.value)"
            >{{ opt.label }}</button>
          </div>
        </div>
      </div>
    </Card>

    <!-- 商品版图 -->
    <Card class="p-5">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-base font-semibold">{{ t('admin.homepage.tilesTitle') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.homepage.tilesDesc') }}</p>
        </div>
        <Button size="sm" variant="outline" :disabled="tiles.length >= 8" @click="addTile">{{ t('admin.homepage.addTile') }}</Button>
      </div>

      <p v-if="!tiles.length" class="mt-4 rounded-lg border border-dashed p-4 text-sm text-muted-foreground">{{ t('admin.homepage.tilesAuto') }}</p>

      <div v-else class="mt-4 space-y-2">
        <div v-for="(tile, index) in tiles" :key="index" class="space-y-3 rounded-lg border border-border p-3">
        <div class="flex flex-wrap items-center gap-3">
          <span class="w-6 text-center text-xs text-muted-foreground">{{ index + 1 }}</span>
          <select v-model.number="tile.product_id" class="h-9 min-w-[220px] flex-1 rounded-md border border-border bg-background px-3 text-sm">
            <option :value="0" disabled>{{ t('admin.homepage.pickProduct') }}</option>
            <option v-for="p in products" :key="p.id" :value="p.id">{{ localized(p.title) }}</option>
          </select>
          <div class="flex gap-1">
            <button
              v-for="opt in tileToneOptions"
              :key="opt.value"
              type="button"
              class="h-8 rounded-md border px-3 text-xs"
              :class="tile.tone === opt.value ? 'border-primary bg-primary/10 text-primary' : 'border-border text-muted-foreground'"
              @click="tile.tone = opt.value as 'dark' | 'light'"
            >{{ opt.label }}</button>
          </div>
          <div class="flex gap-1">
            <button type="button" class="h-8 w-8 rounded-md border border-border text-sm" :disabled="index === 0" @click="move(index, -1)">↑</button>
            <button type="button" class="h-8 w-8 rounded-md border border-border text-sm" :disabled="index === tiles.length - 1" @click="move(index, 1)">↓</button>
            <button type="button" class="h-8 w-8 rounded-md border border-border text-sm" @click="tiles.splice(index, 1)">✕</button>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.desktopImage') }}</div>
            <MediaPicker v-model="tile.image" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.homepage.desktopImageHint') }}</p>
          </div>
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.mobileImage') }}</div>
            <MediaPicker v-model="tile.mobile_image" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.homepage.mobileImageHint') }}</p>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.eyebrowText') }}</div>
            <Input v-model="tile.eyebrow_text" :placeholder="t('admin.homepage.eyebrowTextPlaceholder')" />
          </div>
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.eyebrowColor') }}</div>
            <div class="flex items-center gap-2">
              <input v-model="tile.eyebrow_color" type="color" class="h-9 w-12 cursor-pointer rounded-md border border-border bg-background p-1" />
              <Input v-model="tile.eyebrow_color" class="flex-1" placeholder="#bf4800" />
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.ctaText') }}</div>
            <Input v-model="tile.cta_text" :placeholder="t('admin.homepage.ctaTextPlaceholder')" />
          </div>
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.ctaLink') }}</div>
            <Input v-model="tile.cta_link" :placeholder="t('admin.homepage.ctaLinkPlaceholder')" />
          </div>
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.buyText') }}</div>
            <Input v-model="tile.buy_text" :placeholder="t('admin.homepage.buyTextPlaceholder')" />
          </div>
          <div>
            <div class="mb-1 text-xs font-medium text-muted-foreground">{{ t('admin.homepage.buyLink') }}</div>
            <Input v-model="tile.buy_link" :placeholder="t('admin.homepage.buyLinkPlaceholder')" />
          </div>
        </div>
        </div>
      </div>

      <div class="mt-4 flex items-center gap-3">
        <Button :disabled="saving" @click="save">{{ saving ? t('admin.common.saving') : t('admin.common.save') }}</Button>
        <span v-if="saved" class="text-sm text-primary">{{ t('admin.homepage.saved') }}</span>
      </div>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import MediaPicker from '@/components/admin/MediaPicker.vue'

interface Tile {
  product_id: number
  tone: 'dark' | 'light'
  image: string
  mobile_image: string
  eyebrow_text: string
  eyebrow_color: string
  cta_text: string
  cta_link: string
  buy_text: string
  buy_link: string
}

const { t, locale } = useI18n()
const router = useRouter()

const products = ref<any[]>([])
const banners = ref<any[]>([])
const bannersLoading = ref(true)
const tiles = ref<Tile[]>([])
const saving = ref(false)
const saved = ref(false)

const toneOptions = [
  { value: '', label: t('admin.banners.form.toneAuto') },
  { value: 'dark', label: t('admin.banners.form.toneDark') },
  { value: 'light', label: t('admin.banners.form.toneLight') },
]
const tileToneOptions = [
  { value: 'dark', label: t('admin.banners.form.toneDark') },
  { value: 'light', label: t('admin.banners.form.toneLight') },
]

const localized = (value: any) => {
  if (!value) return ''
  if (typeof value === 'string') return value
  return value[locale.value] || value['zh-CN'] || value['en-US'] || ''
}

const bannerTone = (banner: any) => (banner.tone === 'dark' || banner.tone === 'light' ? banner.tone : '')

const load = async () => {
  try {
    const [productRes, bannerRes, settingRes] = await Promise.all([
      adminAPI.getProducts({ page: 1, page_size: 100 }),
      adminAPI.getBanners({ page: 1, page_size: 50 }),
      adminAPI.getSettings({ key: 'home_layout' }),
    ])
    const pdata = productRes.data?.data
    products.value = Array.isArray(pdata) ? pdata : (pdata?.list || pdata?.items || [])
    const bdata = bannerRes.data?.data
    banners.value = Array.isArray(bdata) ? bdata : (bdata?.list || bdata?.items || [])
    const layout = settingRes.data?.data || {}
    tiles.value = Array.isArray(layout.tiles)
      ? layout.tiles.map((x: any) => ({
          product_id: Number(x.product_id) || 0,
          tone: x.tone === 'light' ? 'light' : 'dark',
          image: x.image || '',
          mobile_image: x.mobile_image || '',
          eyebrow_text: x.eyebrow_text || '',
          eyebrow_color: x.eyebrow_color || '',
          cta_text: x.cta_text || '',
          cta_link: x.cta_link || '',
          buy_text: x.buy_text || '',
          buy_link: x.buy_link || '',
        }))
      : []
  } finally {
    bannersLoading.value = false
  }
}

const addTile = () => {
  if (tiles.value.length >= 8) return
  tiles.value.push({
    product_id: products.value[0]?.id || 0,
    tone: 'dark',
    image: '',
    mobile_image: '',
    eyebrow_text: '',
    eyebrow_color: '',
    cta_text: '',
    cta_link: '',
    buy_text: '',
    buy_link: '',
  })
}

const move = (index: number, delta: number) => {
  const next = index + delta
  if (next < 0 || next >= tiles.value.length) return
  const list = tiles.value
  const a = list[index]
  const b = list[next]
  if (!a || !b) return
  list[index] = b
  list[next] = a
}

const setBannerTone = async (banner: any, tone: string) => {
  const payload = { ...banner, tone }
  const res = await adminAPI.updateBanner(banner.id, payload)
  if (res.data?.status_code === 0) banner.tone = tone
}

const save = async () => {
  saving.value = true
  saved.value = false
  try {
    const res = await adminAPI.updateSettings({
      key: 'home_layout',
      value: { tiles: tiles.value.filter((x) => x.product_id > 0) },
    })
    if (res.data?.status_code === 0) {
      saved.value = true
      setTimeout(() => { saved.value = false }, 2000)
    }
  } catch (error) {
    console.error('save home layout failed:', error)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
