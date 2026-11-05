<template>
  <div class="px">
    <!-- 对比 -->
    <section class="px-sec">
      <button type="button" class="px-head" @click="open.compare = !open.compare">
        <span class="px-chevron" :class="{ 'is-open': open.compare }">⌄</span>
        <strong>{{ t('admin.productExtras.compare') }}</strong>
        <span class="px-hint">{{ t('admin.productExtras.compareHint') }}</span>
      </button>

      <div v-show="open.compare" class="px-body">
        <div class="px-row">
          <label class="px-field">
            <span>{{ t('admin.productExtras.group') }}</span>
            <Select
              :model-value="compare.group_id ? String(compare.group_id) : '__none__'"
              @update:model-value="(v) => selectGroup(v === '__none__' ? 0 : Number(v))"
            >
              <SelectTrigger class="h-9 w-full"><SelectValue :placeholder="t('admin.productExtras.noGroup')" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">{{ t('admin.productExtras.noGroup') }}</SelectItem>
                <SelectItem v-for="group in groups" :key="group.id" :value="String(group.id)">
                  {{ groupName(group) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </label>
          <Button size="sm" variant="outline" type="button" @click="groupsOpen = true">
            {{ t('admin.productExtras.manageGroups') }}
          </Button>
        </div>

        <div class="px-row">
          <label class="px-field">
            <span>{{ t('admin.productExtras.tagline') }}</span>
            <Input :model-value="textField('tagline')" :placeholder="t('admin.productExtras.taglinePlaceholder')" @update:model-value="(v) => setTextField('tagline', String(v))" />
          </label>
          <label class="px-field px-field--narrow">
            <span>{{ t('admin.productExtras.badge') }}</span>
            <Input :model-value="textField('badge')" :placeholder="t('admin.productExtras.badgePlaceholder')" @update:model-value="(v) => setTextField('badge', String(v))" />
          </label>
        </div>

        <div v-if="dimensions.length" class="px-dims">
          <label v-for="dim in dimensions" :key="dim.key" class="px-field px-field--dim">
            <span>{{ dimName(dim) }}<em v-if="dim.unit">（{{ dim.unit }}）</em></span>
            <Input
              :model-value="valueOf(dim.key)"
              :placeholder="t('admin.productExtras.valuePlaceholder')"
              @update:model-value="(v) => setValue(dim.key, String(v))"
            />
            <Textarea
              class="px-detail-input"
              rows="2"
              :model-value="detailOf(dim.key)"
              :placeholder="t('admin.productExtras.detailPlaceholder')"
              @update:model-value="(v) => setDetail(dim.key, String(v))"
            />
          </label>
        </div>

        <div class="px-row">
          <label class="px-field">
            <span>{{ t('admin.productExtras.mode') }}</span>
            <div class="px-seg">
              <button type="button" :class="{ 'is-active': compare.mode !== 'manual' }" @click="compare.mode = 'auto'">
                {{ t('admin.productExtras.modeAuto') }}
              </button>
              <button type="button" :class="{ 'is-active': compare.mode === 'manual' }" @click="compare.mode = 'manual'">
                {{ t('admin.productExtras.modeManual') }}
              </button>
            </div>
          </label>
          <label class="px-field px-field--narrow">
            <span>{{ t('admin.productExtras.limit') }}</span>
            <Input
              type="number"
              min="1"
              max="4"
              :model-value="String(compare.limit || 2)"
              @update:model-value="(v) => (compare.limit = Math.max(1, Math.min(4, Number(v) || 2)))"
            />
          </label>
        </div>

        <div v-if="compare.mode === 'manual'" class="px-field">
          <span>{{ t('admin.productExtras.compareWith') }}</span>
          <Input v-model="productSearch" :placeholder="t('admin.productExtras.searchProduct')" />
          <div v-if="searchResults.length" class="px-pick">
            <button
              v-for="item in searchResults"
              :key="item.id"
              type="button"
              class="px-pick-item"
              :disabled="compareIds.includes(item.id)"
              @click="addCompare(item.id)"
            >{{ getLocalizedText(item.title) }}</button>
          </div>
          <div v-if="compareIds.length" class="px-chips">
            <span v-for="id in compareIds" :key="id" class="px-chip">
              #{{ id }}
              <button type="button" @click="removeCompare(id)">✕</button>
            </span>
          </div>
        </div>

        <div class="px-field">
          <span>{{ t('admin.productExtras.compareImage') }}</span>
          <MediaPicker
            :model-value="compare.image || ''"
            scene="product"
            @update:model-value="(v) => (compare.image = String(v || ''))"
          />
        </div>

        <div class="px-row">
          <label class="px-field px-field--row"><Switch v-model="compare.show_price" /><span>{{ t('admin.productExtras.showPrice') }}</span></label>
          <label class="px-field px-field--row"><Switch v-model="compare.show_cta" /><span>{{ t('admin.productExtras.showCta') }}</span></label>
        </div>
      </div>
    </section>

    <!-- 常见问题 -->
    <section class="px-sec">
      <button type="button" class="px-head" @click="open.faq = !open.faq">
        <span class="px-chevron" :class="{ 'is-open': open.faq }">⌄</span>
        <strong>{{ t('admin.productExtras.faq') }}</strong>
        <span class="px-hint">{{ faqItems.length }} {{ t('admin.productExtras.items') }}</span>
      </button>

      <div v-show="open.faq" class="px-body">
        <div v-for="(item, index) in faqItems" :key="item.id" class="px-item">
          <div class="px-item-head">
            <span class="px-item-index">#{{ index + 1 }}</span>
            <button type="button" class="px-remove" @click="removeFaq(index)">{{ t('admin.common.delete') }}</button>
          </div>
          <label class="px-field">
            <span>{{ t('admin.productExtras.question') }}</span>
            <Input :model-value="item.q[lang] || ''" @update:model-value="(v) => (item.q[lang] = String(v))" />
          </label>
          <label class="px-field">
            <span>{{ t('admin.productExtras.answer') }}</span>
            <RichEditor
              :model-value="item.a[lang] || ''"
              :placeholder="t('admin.productExtras.answerPlaceholder')"
              @update:model-value="(v) => (item.a[lang] = String(v))"
            />
          </label>
        </div>
        <Button size="sm" variant="outline" type="button" @click="addFaq">+ {{ t('admin.productExtras.addFaq') }}</Button>
      </div>
    </section>

    <!-- 包装内容 -->
    <section class="px-sec">
      <button type="button" class="px-head" @click="open.box = !open.box">
        <span class="px-chevron" :class="{ 'is-open': open.box }">⌄</span>
        <strong>{{ t('admin.productExtras.inBox') }}</strong>
        <span class="px-hint">{{ boxItems.length }} {{ t('admin.productExtras.items') }}</span>
      </button>

      <div v-show="open.box" class="px-body">
        <div class="px-field">
          <span>{{ t('admin.productExtras.boxImage') }}</span>
          <MediaPicker :model-value="inBox.image || ''" scene="product" @update:model-value="(v) => (inBox.image = String(v || ''))" />
        </div>
        <div v-for="(item, index) in boxItems" :key="index" class="px-boxitem">
          <MediaPicker
            :model-value="item.image || ''"
            scene="product"
            @update:model-value="(v) => (item.image = String(v || ''))"
          />
          <Input :model-value="item.label[lang] || ''" :placeholder="t('admin.productExtras.boxLabel')" @update:model-value="(v) => (item.label[lang] = String(v))" />
          <button type="button" class="px-remove" @click="removeBoxItem(index)">✕</button>
        </div>
        <Button size="sm" variant="outline" type="button" @click="addBoxItem">+ {{ t('admin.productExtras.addBoxItem') }}</Button>
      </div>
    </section>

    <CompareGroupsDialog v-model="groupsOpen" :lang="lang" :categories="categories" @changed="loadGroups" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import { Textarea } from '@/components/ui/textarea'
import RichEditor from '@/components/RichEditor.vue'
import CompareGroupsDialog from '@/components/admin/blocks/CompareGroupsDialog.vue'
import { getLocalizedText } from '@/utils/format'

interface Dimension { key: string; label: Record<string, string>; unit: string; icon: string; visible?: boolean }
interface Group { id: number; name: Record<string, string>; dimensions?: { items?: Dimension[] } }

const props = defineProps<{
  lang: string
  categories: Array<{ id: number; name: any }>
  compare: Record<string, any>
  faq: Record<string, any>
  inBox: Record<string, any>
}>()

const emit = defineEmits<{
  'update:compare': [Record<string, any>]
  'update:faq': [Record<string, any>]
  'update:inBox': [Record<string, any>]
}>()

const { t } = useI18n()
const open = reactive({ compare: false, faq: false, box: false })
const groupsOpen = ref(false)
const groups = ref<Group[]>([])
const productSearch = ref('')
const searchResults = ref<Array<{ id: number; title: any }>>([])

/* 统一用本地副本 + watch 回写，避免直接改 props */
const compare = computed(() => props.compare || {})
const inBox = computed(() => props.inBox || {})

const patchCompare = (patch: Record<string, any>) => {
  emit('update:compare', { ...compare.value, ...patch })
}

const dimensions = computed<Dimension[]>(() => {
  const group = groups.value.find((g) => g.id === Number(compare.value.group_id))
  return (group?.dimensions?.items || []).filter((d) => d.visible !== false)
})

const compareIds = computed<number[]>(() =>
  Array.isArray(compare.value.compare_with) ? compare.value.compare_with.map(Number) : [],
)

const faqItems = computed<any[]>(() => (props.faq?.items as any[]) || [])
const boxItems = computed<any[]>(() => (inBox.value.items as any[]) || [])

const groupName = (group: Group) => getLocalizedText(group.name) || `#${group.id}`
const dimName = (dim: Dimension) => getLocalizedText(dim.label) || dim.key
const valueOf = (key: string) => {
  const values = compare.value.values || {}
  const entry = values[key]
  if (!entry) return ''
  return typeof entry === 'string' ? entry : entry[props.lang] || entry['zh-CN'] || ''
}

const setValue = (key: string, value: string) => {
  const values = { ...(compare.value.values || {}) }
  const current = values[key]
  values[key] = { ...(typeof current === 'object' && current ? current : {}), [props.lang]: value }
  patchCompare({ values })
}

const textField = (key: 'tagline' | 'badge') => {
  const entry = compare.value[key]
  if (!entry) return ''
  return typeof entry === 'string' ? entry : entry[props.lang] || entry['zh-CN'] || ''
}

const setTextField = (key: 'tagline' | 'badge', value: string) => {
  const current = compare.value[key]
  patchCompare({
    [key]: { ...(typeof current === 'object' && current ? current : {}), [props.lang]: value },
  })
}

const detailOf = (key: string) => {
  const entry = (compare.value.details || {})[key]
  if (!entry) return ''
  return typeof entry === 'string' ? entry : entry[props.lang] || entry['zh-CN'] || ''
}

const setDetail = (key: string, value: string) => {
  const details = { ...(compare.value.details || {}) }
  const current = details[key]
  details[key] = { ...(typeof current === 'object' && current ? current : {}), [props.lang]: value }
  patchCompare({ details })
}

const selectGroup = (id: number) => {
  const group = groups.value.find((g) => g.id === id)
  patchCompare({ group_id: id || null, values: id ? compare.value.values || {} : {} })
  if (!group) return
}

const addCompare = (id: number) => {
  const next = [...compareIds.value, id]
  patchCompare({ compare_with: next })
  productSearch.value = ''
  searchResults.value = []
}

const removeCompare = (id: number) => {
  patchCompare({ compare_with: compareIds.value.filter((item) => item !== id) })
}

const addFaq = () => {
  const items = [...faqItems.value, { id: `q_${Date.now().toString(36)}`, q: {}, a: {} }]
  emit('update:faq', { ...(props.faq || {}), items })
}

const removeFaq = (index: number) => {
  const items = faqItems.value.filter((_, i) => i !== index)
  emit('update:faq', { ...(props.faq || {}), items })
}

const addBoxItem = () => {
  const items = [...boxItems.value, { image: '', label: {} }]
  emit('update:inBox', { ...inBox.value, items })
}

const removeBoxItem = (index: number) => {
  const items = boxItems.value.filter((_, i) => i !== index)
  emit('update:inBox', { ...inBox.value, items })
}

const loadGroups = async () => {
  try {
    const res = await adminAPI.getCompareGroups({ silent: true })
    groups.value = (res.data.data || []) as Group[]
  } catch {
    groups.value = []
  }
}

let searchTimer: ReturnType<typeof setTimeout> | null = null
watch(productSearch, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    const keyword = productSearch.value.trim()
    if (!keyword) {
      searchResults.value = []
      return
    }
    try {
      const res = await adminAPI.getProducts({ search: keyword, page: 1, page_size: 10 })
      searchResults.value = ((res.data.data || []) as any[]).map((p) => ({ id: p.id, title: p.title }))
    } catch {
      searchResults.value = []
    }
  }, 300)
})

onMounted(loadGroups)
</script>

<style scoped>
.px { margin-top: 8px; }
.px-sec {
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 10px;
  margin-bottom: 10px;
  overflow: hidden;
}
.px-head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 10px 12px;
  border: 0;
  background: var(--muted, #f9fafb);
  text-align: left;
  cursor: pointer;
}
.px-chevron {
  display: inline-block;
  color: var(--muted-foreground, #9ca3af);
  transform: rotate(-90deg);
  transition: transform 160ms ease;
}
.px-chevron.is-open { transform: rotate(0deg); }
.px-hint { margin-left: auto; font-size: 11px; color: var(--muted-foreground, #9ca3af); }
.px-body { padding: 12px; }
.px-row {
  display: flex;
  gap: 10px;
  align-items: flex-end;
  margin-bottom: 10px;
}
.px-field { flex: 1; min-width: 0; margin-bottom: 10px; }
.px-field > span {
  display: block;
  margin-bottom: 5px;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
}
.px-field--row { display: flex; align-items: center; gap: 8px; }
.px-field--row > span { margin: 0; }
.px-field--narrow { max-width: 90px; }
.px-field--dim { margin-bottom: 10px; }
.px-detail-input { margin-top: 5px; font-size: 12px; }
.px-dims {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 10px;
}
.px-seg {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--muted, #f3f4f6);
}
.px-seg button {
  height: 26px;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  background: none;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.px-seg button.is-active {
  background: var(--background, #fff);
  color: var(--foreground, #111827);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}
.px-pick { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; }
.px-pick-item {
  height: 26px;
  padding: 0 10px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 999px;
  background: none;
  font-size: 12px;
  cursor: pointer;
}
.px-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; }
.px-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--muted, #f3f4f6);
  font-size: 12px;
}
.px-chip button { border: 0; background: none; color: #dc2626; cursor: pointer; }
.px-item {
  padding: 10px;
  border: 1px solid var(--border, #f1f5f9);
  border-radius: 8px;
  margin-bottom: 8px;
}
.px-item-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
.px-item-index { font-size: 11px; color: var(--muted-foreground, #9ca3af); }
.px-remove { border: 0; background: none; color: #dc2626; font-size: 12px; cursor: pointer; }
.px-boxitem {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 24px;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
</style>
