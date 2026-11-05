<template>
  <div class="pe">
    <!-- 顶栏：标题 + 语言 + 状态 + 保存，永远在视线里 -->
    <div class="pe-bar">
      <button type="button" class="pe-back" :title="t('admin.postEdit.back')" @click="goBack">←</button>

      <input
        v-model="form.title[lang]"
        class="pe-title"
        :placeholder="t('admin.posts.form.titlePlaceholder')"
        :aria-label="t('admin.posts.form.title', { lang: currentLangName })"
      />

      <span class="pe-status" :class="{ 'is-live': form.is_published }">
        {{ form.is_published ? t('admin.posts.status.published') : t('admin.posts.status.draft') }}
      </span>
      <span v-if="dirty" class="pe-unsaved">{{ t('admin.postEdit.unsaved') }}</span>
      <span v-else-if="draftAt" class="pe-draft">{{ t('admin.postEdit.draftSaved', { time: draftAt }) }}</span>

      <div class="pe-bar-right">
        <div class="pe-langs">
          <button
            v-for="l in languages"
            :key="l.code"
            type="button"
            class="pe-lang"
            :class="{ 'is-active': lang === l.code }"
            @click="lang = l.code as Lang"
          >{{ l.short }}</button>
        </div>

        <Button v-if="form.layout === 'blocks'" size="sm" variant="ghost" type="button" :disabled="!canUndo" @click="panel?.undo()">
          {{ t('admin.blocks.editor.undo') }}
        </Button>
        <Button size="sm" variant="outline" type="button" @click="openMeta = !openMeta">
          {{ t('admin.postEdit.meta') }}
        </Button>
        <Button size="sm" type="button" :disabled="saving" @click="save">
          {{ saving ? t('admin.common.submitting') : t('admin.posts.actions.saveChanges') }}
        </Button>
      </div>
    </div>

    <div v-if="draftFound" class="pe-restore">
      <span>{{ t('admin.postEdit.draftFound', { time: draftFound.at }) }}</span>
      <button type="button" class="pe-restore-btn" @click="restoreDraft">{{ t('admin.postEdit.draftRestore') }}</button>
      <button type="button" class="pe-restore-skip" @click="discardDraft">{{ t('admin.postEdit.draftDiscard') }}</button>
    </div>

    <div v-if="loading" class="pe-loading">{{ t('admin.common.loading') }}</div>

    <div v-else class="pe-main">
      <!-- 左：真预览 -->
      <div ref="canvasEl" class="pe-canvas">
        <div class="pe-canvas-bar">
          <span class="pe-canvas-label">
            {{ t('admin.postEdit.preview') }}
            <em class="pe-zoom">{{ deviceWidth }}px · {{ Math.round(frameScale * 100) }}%</em>
          </span>
          <div class="pe-canvas-tools">
            <div class="pe-devices">
              <button
                v-for="d in devices"
                :key="d.key"
                type="button"
                class="pe-device"
                :class="{ 'is-active': device === d.key }"
                :title="t(`admin.postEdit.device.${d.key}`)"
                @click="device = d.key"
              >
                <component :is="d.icon" class="h-3.5 w-3.5" />
              </button>
            </div>
            <button type="button" :class="{ 'is-active': !previewDark }" :title="t('admin.postEdit.light')" @click="previewDark = false">☀</button>
            <button type="button" :class="{ 'is-active': previewDark }" :title="t('admin.postEdit.dark')" @click="previewDark = true">☾</button>
            <button type="button" :title="t('admin.postEdit.reload')" @click="reloadPreview">⟳</button>
          </div>
        </div>
        <div class="pe-frame-wrap" :class="`is-${device}`">
          <iframe
            ref="frame"
            :src="previewSrc"
            class="pe-frame"
            :style="frameStyle"
            @load="onFrameLoad"
          />
        </div>
      </div>

      <!-- 右：文章信息 + 区块 -->
      <aside class="pe-side">
        <details v-if="openMeta" class="pe-meta" open>
          <summary>{{ t('admin.postEdit.meta') }}</summary>

          <label class="pe-field">
            <span>{{ t('admin.posts.form.slug') }}</span>
            <Input v-model="form.slug" :placeholder="t('admin.posts.form.slugPlaceholder')" />
            <em v-if="errors.slug" class="pe-error">{{ errors.slug }}</em>
          </label>

          <div class="pe-row">
            <label class="pe-field">
              <span>{{ t('admin.posts.form.type') }}</span>
              <Select v-model="form.type">
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="blog">{{ t('admin.posts.form.typeBlog') }}</SelectItem>
                  <SelectItem value="notice">{{ t('admin.posts.form.typeNotice') }}</SelectItem>
                  <SelectItem value="page">{{ t('admin.posts.form.typePage') }}</SelectItem>
                </SelectContent>
              </Select>
            </label>

            <label v-if="form.type === 'blog'" class="pe-field">
              <span>{{ t('admin.posts.form.category') }}</span>
              <Select
                :model-value="form.category_id != null ? String(form.category_id) : '__none__'"
                @update:model-value="(v) => { form.category_id = v === '__none__' ? null : Number(v) || null }"
              >
                <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.posts.form.categoryPlaceholder')" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">{{ t('admin.posts.form.noCategory') }}</SelectItem>
                  <SelectItem
                    v-for="cat in categories"
                    :key="cat.id"
                    :value="String(cat.id)"
                    :disabled="!cat.selectable"
                  >{{ getLocalizedText(cat.name) }}</SelectItem>
                </SelectContent>
              </Select>
            </label>
          </div>

          <label class="pe-field">
            <span>{{ t('admin.posts.form.summary', { lang: currentLangName }) }}</span>
            <Textarea v-model="form.summary[lang]" rows="2" :placeholder="t('admin.posts.form.summaryPlaceholder')" />
          </label>

          <div class="pe-field">
            <span>{{ t('admin.posts.form.thumbnail') }}</span>
            <MediaPicker v-model="form.thumbnail" scene="post" />
          </div>

          <label class="pe-field pe-field--row">
            <Switch v-model="form.is_published" />
            <span>{{ t('admin.posts.form.publishNow') }}</span>
          </label>

          <div v-if="form.type === 'blog'" class="pe-field">
            <span>{{ t('admin.posts.form.relatedProducts') }}</span>
            <p class="pe-hint">{{ t('admin.posts.form.relatedProductsHint') }}</p>
            <div v-if="relatedProducts.length" class="pe-related">
              <div v-for="p in relatedProducts" :key="p.id" class="pe-related-item">
                <img v-if="p.image" :src="getImageUrl(p.image)" alt="" />
                <span class="truncate">{{ getLocalizedText(p.title) }}</span>
                <button type="button" class="pe-related-remove" @click="removeRelatedProduct(p.id)">✕</button>
              </div>
            </div>
            <Input v-model="productSearch" :placeholder="t('admin.posts.form.relatedProductsSearch')" />
            <div v-if="productSearchResults.length" class="pe-related">
              <div v-for="p in productSearchResults" :key="p.id" class="pe-related-item">
                <img v-if="p.image" :src="getImageUrl(p.image)" alt="" />
                <span class="truncate">{{ getLocalizedText(p.title) }}</span>
                <button
                  type="button"
                  class="pe-related-add"
                  :disabled="form.product_ids.includes(p.id)"
                  @click="addRelatedProduct(p)"
                >{{ t('admin.posts.form.relatedProductsAdd') }}</button>
              </div>
            </div>
          </div>

          <label class="pe-field">
            <span>{{ t('admin.posts.form.content', { lang: currentLangName }) }}</span>
            <div class="pe-row pe-row--tight">
              <button
                v-for="opt in layoutOptions"
                :key="opt.value"
                type="button"
                class="pe-chip"
                :class="{ 'is-active': form.layout === opt.value }"
                @click="form.layout = opt.value as 'article' | 'blocks'"
              >{{ opt.label }}</button>
            </div>
            <RichEditor
              v-if="form.layout === 'article'"
              :model-value="form.content[lang] || ''"
              :placeholder="t('admin.posts.form.contentPlaceholder')"
              @update:model-value="(v: string) => (form.content[lang] = v)"
            />
          </label>
        </details>

        <div v-if="form.layout === 'blocks'" class="pe-blocks">
          <BlockEditor
            ref="panel"
            v-model="form.content_blocks"
            v-model:selectedId="selectedId"
            :lang="lang"
            @update:lang="lang = $event as Lang"
          />
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminPost, AdminProduct, LocalizedText } from '@/api/types'
import BlockEditor from '@/components/admin/BlockEditor.vue'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import RichEditor from '@/components/RichEditor.vue'
import { Monitor, Smartphone, Tablet } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFormValidation, rules } from '@/composables/useFormValidation'
import { getImageUrl } from '@/utils/image'
import { getLocalizedText } from '@/utils/format'
import { notifyError, notifySuccess } from '@/utils/notify'
import { dropCachedPost, readCachedPost } from '@/lib/postCache'

type Lang = 'zh-CN' | 'zh-TW' | 'en-US'

interface RelatedProductRef {
  id: number
  slug: string
  title: LocalizedText
  image?: string
}

interface CategoryItem {
  id: number
  name: LocalizedText
  depth: number
  selectable: boolean
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const languages = computed(() => [
  { code: 'zh-CN', name: t('admin.common.lang.zhCN'), short: '简' },
  { code: 'zh-TW', name: t('admin.common.lang.zhTW'), short: '繁' },
  { code: 'en-US', name: t('admin.common.lang.enUS'), short: 'EN' },
])

const lang = ref<Lang>('zh-CN')
const currentLangName = computed(() => languages.value.find((l) => l.code === lang.value)?.name || '')

const loading = ref(true)
const saving = ref(false)
const openMeta = ref(false)
const selectedId = ref<string | null>(null)
const panel = ref<InstanceType<typeof BlockEditor> | null>(null)

const isNew = computed(() => route.params.id === undefined || route.params.id === 'new')
const postId = computed(() => (isNew.value ? 0 : Number(route.params.id)))

const form = reactive({
  id: 0,
  title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<string, string>,
  slug: '',
  summary: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<string, string>,
  content: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<string, string>,
  layout: 'article' as 'article' | 'blocks',
  content_blocks: { version: 1, blocks: [] as any[] } as any,
  type: 'blog',
  thumbnail: '',
  is_published: true,
  category_id: null as number | null,
  product_ids: [] as number[],
})

const { errors, validate } = useFormValidation({
  slug: [rules.required('This field is required')],
  type: [rules.required('This field is required')],
  title: [rules.required('This field is required')],
})

/* ---------------- 数据 ---------------- */
const categories = ref<CategoryItem[]>([])
const relatedProducts = ref<RelatedProductRef[]>([])
const productSearch = ref('')
const productSearchResults = ref<RelatedProductRef[]>([])
let searchTimer: ReturnType<typeof setTimeout> | null = null

const fetchCategories = async () => {
  try {
    const res = await adminAPI.getPostCategories()
    const flat: any[] = res.data?.data || []
    categories.value = flat.map((c: any) => ({
      id: c.id,
      name: c.name,
      depth: 0,
      selectable: c.is_active !== false,
    }))
  } catch {
    categories.value = []
  }
}

const fetchRelated = async (id: number) => {
  try {
    const res = await adminAPI.getPostRelatedProducts(id)
    relatedProducts.value = (res.data.data || []) as RelatedProductRef[]
    form.product_ids = relatedProducts.value.map((p) => p.id)
  } catch {
    relatedProducts.value = []
    form.product_ids = []
  }
}

const runSearch = async () => {
  const keyword = productSearch.value.trim()
  if (!keyword) {
    productSearchResults.value = []
    return
  }
  try {
    const res = await adminAPI.getProducts({ search: keyword, page: 1, page_size: 20 })
    productSearchResults.value = ((res.data.data || []) as AdminProduct[]).map((p) => ({
      id: p.id,
      slug: p.slug,
      title: p.title,
      image: p.images?.[0],
    }))
  } catch {
    productSearchResults.value = []
  }
}

watch(productSearch, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(runSearch, 300)
})

const addRelatedProduct = (p: RelatedProductRef) => {
  if (form.product_ids.includes(p.id)) return
  relatedProducts.value.push(p)
  form.product_ids.push(p.id)
}
const removeRelatedProduct = (id: number) => {
  relatedProducts.value = relatedProducts.value.filter((p) => p.id !== id)
  form.product_ids = form.product_ids.filter((pid) => pid !== id)
}

/**
 * 取单篇：从列表点进来时数据已经在 history.state 里（秒开、无请求）；
 * 直接访问 URL / 刷新时回落到列表接口里找（后端目前没有 GET /admin/posts/:id）。
 */
const fetchPost = async (id: number): Promise<AdminPost> => {
  const cached = readCachedPost(id)
  if (cached) return cached
  // 后端有 GET /admin/posts/:id 就用它；老版本返回 404 时回落到列表里找
  try {
    const res = await adminAPI.getPost(id, { silent: true })
    const post = (res.data as any)?.data || res.data
    if (post?.id) return post as AdminPost
  } catch {
    /* 老后端没有单篇接口 */
  }
  const res = await adminAPI.getPosts({ page: 1, page_size: 100, type: String(route.params.type || 'blog') })
  const list = (res.data.data || []) as AdminPost[]
  const found = list.find((item) => item.id === id)
  if (!found) throw new Error(t('admin.postEdit.notFound'))
  return found
}

const load = async () => {
  loading.value = true
  try {
    await fetchCategories()
    if (isNew.value) {
      form.type = String(route.params.type || 'blog')
      form.layout = 'blocks'
      openMeta.value = true
      return
    }
    const post = await fetchPost(postId.value)
    Object.assign(form, {
      id: post.id,
      title: post.title || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
      slug: post.slug,
      summary: post.summary || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
      content: post.content || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
      layout: ((post as any).layout === 'blocks' ? 'blocks' : 'article') as 'article' | 'blocks',
      content_blocks: (post as any).content_blocks || { version: 1, blocks: [] },
      type: post.type,
      thumbnail: post.thumbnail || '',
      is_published: post.is_published,
      category_id: post.category_id ?? null,
      product_ids: [],
    })
    // 富文本页没有区块面板，正文就在文章信息里，直接展开
    if (form.layout === 'article') openMeta.value = true
    if (post.type === 'blog') await fetchRelated(post.id)
  } catch (err) {
    notifyError((err as Error).message)
  } finally {
    loading.value = false
    markSaved()
    checkDraft()
  }
}

/* ---------------- 未保存 ---------------- */
const snapshot = ref('')
const formSnapshot = () => JSON.stringify({ ...form, product_ids: [...form.product_ids] })
const markSaved = () => { snapshot.value = formSnapshot() }
const dirty = computed(() => snapshot.value !== '' && formSnapshot() !== snapshot.value)

const goBack = () => {
  router.push(`/posts/${form.type || route.params.type || 'blog'}`)
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm(t('admin.posts.modal.discardConfirm'))
})

const onBeforeUnload = (e: BeforeUnloadEvent) => {
  if (!dirty.value) return
  e.preventDefault()
  e.returnValue = ''
}

/* ---------------- 本地草稿（防手滑关页面） ---------------- */
const draftKey = computed(() => `dj:post-draft:${isNew.value ? 'new-' + (route.params.type || 'blog') : postId.value}`)
const draftAt = ref('')
const draftFound = ref<{ at: string; data: any } | null>(null)
let draftTimer: ReturnType<typeof setTimeout> | null = null

const draftPayload = () => JSON.parse(JSON.stringify(form))

const writeDraft = () => {
  if (!dirty.value) return
  try {
    const at = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    localStorage.setItem(draftKey.value, JSON.stringify({ at, data: draftPayload() }))
    draftAt.value = at
  } catch {
    /* 隐私模式/超额就跳过，别打扰用户 */
  }
}

const clearDraft = () => {
  try { localStorage.removeItem(draftKey.value) } catch { /* ignore */ }
  draftAt.value = ''
  draftFound.value = null
}

const restoreDraft = () => {
  const draft = draftFound.value
  if (!draft) return
  Object.assign(form, draft.data)
  draftFound.value = null
  draftAt.value = draft.at
}

const discardDraft = () => {
  clearDraft()
}

/** 改动停 1.5s 就落一份本地草稿 */
watch(dirty, (value) => {
  if (!value) return
  if (draftTimer) clearTimeout(draftTimer)
  draftTimer = setTimeout(writeDraft, 1500)
})

const checkDraft = () => {
  try {
    const raw = localStorage.getItem(draftKey.value)
    if (!raw) return
    const parsed = JSON.parse(raw)
    if (!parsed?.data) return
    // 和当前表单一样就没必要提示
    if (JSON.stringify(parsed.data) === JSON.stringify(draftPayload())) {
      clearDraft()
      return
    }
    draftFound.value = { at: parsed.at || '', data: parsed.data }
  } catch {
    /* ignore */
  }
}

/* ---------------- 保存 ---------------- */
const save = async () => {
  if (!validate({ slug: form.slug, type: form.type, title: form.title['zh-CN'] })) {
    openMeta.value = true
    return
  }
  saving.value = true
  try {
    const payload: any = {
      ...form,
      category_id: form.type === 'blog' ? form.category_id : null,
      product_ids: form.type === 'blog' ? [...form.product_ids] : [],
    }
    if (isNew.value) {
      const res = await adminAPI.createPost(payload)
      const created = (res.data?.data || res.data) as AdminPost
      markSaved()
      clearDraft()
      notifySuccess(t('admin.postEdit.saved'))
      if (created?.id) {
        router.replace(`/posts/${payload.type}/${created.id}`)
      } else {
        goBack()
      }
    } else {
      await adminAPI.updatePost(form.id, payload)
      dropCachedPost(form.id)
      markSaved()
      clearDraft()
      notifySuccess(t('admin.postEdit.saved'))
    }
  } catch (err) {
    notifyError((err as Error).message)
  } finally {
    saving.value = false
  }
}

/* ---------------- 预览 ---------------- */
const frame = ref<HTMLIFrameElement | null>(null)
const canvasEl = ref<HTMLElement | null>(null)
const previewDark = ref(false)

/** 预览按真实设备宽度渲染，再等比缩到画布里 */
const devices = [
  { key: 'desktop' as const, width: 1440, icon: Monitor },
  { key: 'tablet' as const, width: 834, icon: Tablet },
  { key: 'mobile' as const, width: 390, icon: Smartphone },
]
const device = ref<'desktop' | 'tablet' | 'mobile'>('desktop')
const deviceWidth = computed(() => devices.find((d) => d.key === device.value)?.width ?? 1440)
const frameScale = ref(1)
const frameHeight = ref(900)
let resizeObserver: ResizeObserver | null = null

const frameStyle = computed(() => ({
  width: `${deviceWidth.value}px`,
  height: `${frameHeight.value}px`,
  transform: `scale(${frameScale.value})`,
}))

const measureCanvas = () => {
  const el = canvasEl.value
  if (!el) return
  const availableW = el.clientWidth
  const availableH = el.clientHeight - 36 // 减去画布顶部工具条
  if (!availableW || !availableH) return
  const scale = Math.min(1, availableW / deviceWidth.value)
  frameScale.value = scale
  frameHeight.value = Math.round(availableH / scale)
}

const previewSrc = computed(() => {
  const override = import.meta.env.VITE_PREVIEW_ORIGIN
  if (override) return `${String(override).replace(/\/$/, '')}/preview/blocks`
  if (import.meta.env.DEV) {
    const host = window.location.hostname || '127.0.0.1'
    return `http://${host}:5173/preview/blocks`
  }
  return `${window.location.origin}/preview/blocks`
})

const pushPreview = (reveal: string | null = null) => {
  const win = frame.value?.contentWindow
  if (!win) return
  // 必须转成纯 JSON：Vue 的 reactive proxy 过不了 structured clone
  const blocks = JSON.parse(JSON.stringify(form.content_blocks?.blocks || []))
  win.postMessage(
    {
      type: 'dj:preview',
      blocks,
      locale: lang.value,
      dark: previewDark.value,
      selected: selectedId.value,
      reveal,
    },
    '*',
  )
}

let pushTimer: ReturnType<typeof setTimeout> | null = null
const schedulePush = () => {
  if (pushTimer) clearTimeout(pushTimer)
  pushTimer = setTimeout(() => pushPreview(), 120)
}

const onFrameLoad = () => {
  pushPreview()
  // iframe 里 Vue 挂载完可能晚于 load，再补一枪
  setTimeout(pushPreview, 400)
}

const reloadPreview = () => {
  if (frame.value) frame.value.src = previewSrc.value
}

const onMessage = (event: MessageEvent) => {
  const data = event.data
  if (!data || typeof data !== 'object') return
  if (data.type === 'dj:preview-ready') pushPreview()
  if (data.type === 'dj:preview-select' && data.id) {
    fromCanvas = true
    selectedId.value = String(data.id)
    // 画布里点的，把选中态回推给预览（不滚动）
    pushPreview()
  }
}

/** 面板里选中时，让预览把该区块滚到中间 */
let fromCanvas = false
watch(selectedId, (id) => {
  if (fromCanvas) {
    fromCanvas = false
    return
  }
  if (id) pushPreview(id)
})

watch([() => form.content_blocks, () => lang.value, previewDark], schedulePush, { deep: true })

watch(device, async () => {
  await nextTick()
  measureCanvas()
  setTimeout(pushPreview, 260)
})

watch(canvasEl, (el) => {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (!el) return
  if ('ResizeObserver' in window) {
    resizeObserver = new ResizeObserver(() => measureCanvas())
    resizeObserver.observe(el)
  }
  measureCanvas()
})

onMounted(() => {
  window.addEventListener('message', onMessage)
  window.addEventListener('beforeunload', onBeforeUnload)
  load()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  window.removeEventListener('message', onMessage)
  window.removeEventListener('beforeunload', onBeforeUnload)
  if (pushTimer) clearTimeout(pushTimer)
  if (searchTimer) clearTimeout(searchTimer)
  if (draftTimer) clearTimeout(draftTimer)
})

const layoutOptions = computed(() => [
  { value: 'article', label: t('admin.blocks.editor.layoutArticle') },
  { value: 'blocks', label: t('admin.blocks.editor.layoutBlocks') },
])

const canUndo = computed(() => {
  const p: any = panel.value
  return Boolean(p?.canUndo)
})
</script>

<style scoped>
.pe {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 7.5rem);
  min-height: 520px;
}

/* ---------- 顶栏 ---------- */
.pe-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 4px 10px;
}
.pe-back {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex: none;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 8px;
  background: var(--background, #fff);
  font-size: 15px;
  cursor: pointer;
}
.pe-title {
  flex: 1;
  min-width: 0;
  height: 36px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  font-size: 17px;
  font-weight: 600;
  color: var(--foreground, #111827);
}
.pe-title:hover {
  border-color: var(--border, #e5e7eb);
}
.pe-title:focus {
  border-color: var(--foreground, #111827);
  outline: none;
  background: var(--background, #fff);
}
.pe-status {
  flex: none;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--muted, #f3f4f6);
  color: var(--muted-foreground, #6b7280);
  font-size: 11px;
}
.pe-status.is-live {
  background: rgba(0, 128, 9, 0.1);
  color: #008009;
}
.pe-draft {
  flex: none;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.pe-restore {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
  padding: 8px 12px;
  border: 1px solid color-mix(in srgb, #b45309 35%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, #b45309 10%, var(--background, #fff));
  font-size: 12px;
  color: color-mix(in srgb, #b45309 85%, var(--foreground, #111827));
}
.pe-restore-btn,
.pe-restore-skip {
  height: 24px;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid currentColor;
  background: none;
  font-size: 12px;
  color: inherit;
  cursor: pointer;
}
.pe-restore-skip {
  border-color: transparent;
  opacity: 0.7;
}
.pe-unsaved {
  flex: none;
  font-size: 11px;
  color: #b45309;
}
.pe-bar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}
.pe-langs {
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--muted, #f3f4f6);
}
.pe-lang {
  min-width: 32px;
  height: 26px;
  border: 0;
  border-radius: 6px;
  background: none;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.pe-lang.is-active {
  background: var(--background, #fff);
  color: var(--foreground, #111827);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}

/* ---------- 主体 ---------- */
.pe-main {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: 14px;
}
.pe-canvas {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 12px;
  background: var(--muted, #f5f5f7);
}
.pe-canvas-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 36px;
  padding: 0 10px;
  border-bottom: 1px solid var(--border, #e5e7eb);
  background: var(--background, #fff);
}
.pe-canvas-label {
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
}
.pe-canvas-tools {
  display: flex;
  gap: 4px;
  align-items: center;
}
.pe-devices {
  display: flex;
  gap: 2px;
  margin-right: 6px;
  padding: 2px;
  border-radius: 8px;
  background: var(--muted, #f3f4f6);
}
.pe-device {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 22px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.pe-device.is-active {
  background: var(--background, #fff);
  color: var(--foreground, #111827);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}
.pe-canvas-tools button {
  width: 26px;
  height: 24px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 6px;
  background: var(--background, #fff);
  font-size: 12px;
  cursor: pointer;
}
.pe-canvas-tools button.is-active {
  border-color: var(--foreground, #111827);
  color: var(--foreground, #111827);
}
.pe-frame-wrap {
  position: relative;
  display: flex;
  flex: 1;
  align-items: flex-start;
  justify-content: center;
  overflow: hidden;
  background: var(--muted, #f5f5f7);
}
.pe-frame-wrap.is-mobile .pe-frame,
.pe-frame-wrap.is-tablet .pe-frame {
  border-radius: 10px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
}
.pe-frame {
  flex: none;
  border: 0;
  background: #fff;
  transform-origin: top center;
}
.pe-zoom {
  margin-left: 6px;
  font-style: normal;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}

.pe-side {
  display: flex;
  flex: none;
  width: 430px;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
  padding-right: 2px;
}

/* ---------- 文章信息 ---------- */
.pe-meta {
  flex: none;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 12px;
  padding: 12px 14px 4px;
}
.pe-meta summary {
  margin-bottom: 12px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}
.pe-field {
  display: block;
  margin-bottom: 12px;
}
.pe-field > span {
  display: block;
  margin-bottom: 5px;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
}
.pe-field--row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.pe-field--row > span {
  margin: 0;
}
.pe-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.pe-row--tight {
  display: flex;
  gap: 6px;
}
.pe-chip {
  height: 30px;
  padding: 0 12px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 8px;
  background: none;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.pe-chip.is-active {
  border-color: var(--primary, #2563eb);
  background: rgba(37, 99, 235, 0.08);
  color: var(--primary, #2563eb);
}
.pe-error {
  display: block;
  margin-top: 4px;
  font-size: 11px;
  font-style: normal;
  color: #dc2626;
}
.pe-hint {
  margin: 0 0 6px;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.pe-related {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 6px 0;
}
.pe-related-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 6px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 8px;
  font-size: 12px;
}
.pe-related-item img {
  width: 28px;
  height: 28px;
  flex: none;
  border-radius: 5px;
  object-fit: cover;
}
.pe-related-item .truncate {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pe-related-add,
.pe-related-remove {
  flex: none;
  border: 0;
  background: none;
  font-size: 12px;
  color: var(--primary, #2563eb);
  cursor: pointer;
}
.pe-related-remove {
  color: #dc2626;
}
.pe-blocks {
  flex: 1;
  min-height: 0;
}
.pe-loading {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: var(--muted-foreground, #9ca3af);
}

@media (max-width: 1023px) {
  .pe {
    height: auto;
  }
  .pe-main {
    flex-direction: column;
  }
  .pe-canvas {
    height: 60vh;
  }
  .pe-side {
    width: 100%;
  }
}
</style>
