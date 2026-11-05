<template>
  <div ref="rootEl" class="be" @dragover="onRootDragOver">
    <div class="be-head">
      <div class="be-hint">{{ t('admin.blocks.editor.blockCount', { count: blocks.length }) }}</div>
      <div class="be-head-actions">
        <Button v-if="blocks.length" size="sm" variant="outline" type="button" @click="recipeOpen = !recipeOpen">
          {{ t('admin.blocks.editor.useRecipe') }}
        </Button>
        <Button
          v-if="clipboard"
          size="sm"
          variant="outline"
          type="button"
          :title="t('admin.blocks.editor.pasteHint')"
          @click="pasteBlock"
        >{{ t('admin.blocks.editor.pasteBlock') }}</Button>
        <Button size="sm" type="button" @click="openPicker(blocks.length)">
          + {{ t('admin.blocks.editor.addBlock') }}
        </Button>
      </div>
    </div>

    <div v-if="recipeOpen" class="be-picker">
      <button
        v-for="recipe in blockRecipes"
        :key="recipe.key"
        type="button"
        class="be-pick"
        @click="applyRecipe(recipe, true)"
      >
        <span class="be-pick-name">{{ t(`admin.blocks.recipes.${recipe.labelKey}`) }}</span>
        <span class="be-pick-desc">{{ t(`admin.blocks.recipes.${recipe.descKey}`) }}</span>
      </button>
    </div>

    <div v-if="!blocks.length" class="be-recipes">
      <p class="be-recipes-title">{{ t('admin.blocks.editor.pickRecipe') }}</p>
      <div class="be-recipes-grid">
        <button
          v-for="recipe in blockRecipes"
          :key="recipe.key"
          type="button"
          class="be-recipe"
          @click="applyRecipe(recipe)"
        >
          <span class="be-recipe-name">{{ t(`admin.blocks.recipes.${recipe.labelKey}`) }}</span>
          <span class="be-recipe-desc">{{ t(`admin.blocks.recipes.${recipe.descKey}`) }}</span>
        </button>
      </div>
      <p class="be-hint">{{ t('admin.blocks.editor.empty') }}</p>
    </div>

    <template v-for="(block, index) in blocks" :key="block.id">
      <!-- 就地插入：悬停才显示的一条细线 -->
      <div class="be-slot">
        <button
          v-if="pickerIndex !== index"
          type="button"
          class="be-inserter"
          @click="openPicker(index)"
        >+ {{ t('admin.blocks.editor.insertHere') }}</button>
        <div v-else class="be-picker be-picker--grid">
          <button
            v-for="def in blockDefs"
            :key="def.type"
            type="button"
            class="be-type-card"
            @click="insertBlock(def, index)"
          >
            <span class="be-type-thumb"><BlockPreview :type="def.type" variant="thumb" /></span>
            <span class="be-type-name">{{ t(`admin.blocks.types.${def.labelKey}`) }}</span>
            <span class="be-type-desc">{{ t(`admin.blocks.typeDescs.${def.type}`) }}</span>
          </button>
          <button type="button" class="be-picker-cancel" @click="pickerIndex = null">{{ t('admin.blocks.editor.cancel') }}</button>
        </div>
      </div>

      <div
        class="be-block"
        :data-block-id="block.id"
        :class="{
          'is-open': openId === block.id,
          'is-dragging': dragIndex === index,
          'is-drop-target': overIndex === index && dragIndex !== index && dragIndex !== null,
        }"
        @dragover.prevent="onDragOver(index)"
        @drop.prevent="onDrop(index)"
      >
        <div class="be-block-head">
          <span
            class="be-grip"
            role="button"
            tabindex="0"
            draggable="true"
            :aria-label="t('admin.blocks.editor.insertHere')"
            @dragstart="onDragStart(index, $event)"
            @dragend="resetDrag"
          ><GripVertical class="h-3.5 w-3.5" /></span>
          <button type="button" class="be-toggle" @click="toggle(block.id)">
            <span class="be-index">{{ index + 1 }}</span>
            <span class="be-thumb"><BlockPreview :type="block.type" :props="block.props" variant="thumb" /></span>
            <span class="be-block-title">
              {{ typeLabel(block.type) }}
              <span v-if="summary(block)" class="be-summary">{{ summary(block) }}</span>
            </span>
          </button>
          <div class="be-actions">
            <button type="button" :title="t('admin.blocks.editor.moveUp')" :disabled="index === 0" @click="move(index, -1)"><ArrowUp class="h-3.5 w-3.5" /></button>
            <button type="button" :title="t('admin.blocks.editor.moveDown')" :disabled="index === blocks.length - 1" @click="move(index, 1)"><ArrowDown class="h-3.5 w-3.5" /></button>
            <button type="button" :title="t('admin.blocks.editor.duplicate')" @click="duplicate(index)"><Copy class="h-3.5 w-3.5" /></button>
            <button type="button" :title="t('admin.blocks.editor.copyBlock')" @click="copyBlock(index)"><ClipboardCopy class="h-3.5 w-3.5" /></button>
            <button type="button" class="danger" :title="t('admin.blocks.editor.remove')" @click="remove(index)"><Trash2 class="h-3.5 w-3.5" /></button>
          </div>
          <span class="be-chevron" :class="{ 'is-open': openId === block.id }"><ChevronDown class="h-4 w-4" /></span>
        </div>

        <div v-if="openId === block.id" class="be-block-body">
          <div
            v-for="group in FIELD_GROUPS"
            :key="group"
            class="be-group"
            :class="{ 'is-collapsed': group !== 'content' && !isGroupOpen(block.id, group) }"
          >
            <template v-if="fieldsInGroup(block.type, group).length">
              <button
                v-if="group !== 'content'"
                type="button"
                class="be-group-head"
                @click="toggleGroup(block.id, group)"
              >
                <ChevronDown class="be-group-chevron" :class="{ 'is-open': isGroupOpen(block.id, group) }" />
                {{ t(`admin.blocks.editor.group.${group}`) }}
                <span class="be-group-count">{{ fieldsInGroup(block.type, group).length }}</span>
              </button>
              <div v-if="group === 'content' || isGroupOpen(block.id, group)" class="be-group-body">
                <BlockField
                  v-for="field in fieldsInGroup(block.type, group)"
                  :key="field.key"
                  :field="field"
                  :model-value="block.props[field.key]"
                  @update:model-value="(v) => (block.props[field.key] = v)"
                />
              </div>
            </template>
          </div>
          <p v-if="!fieldsOf(block.type).length" class="be-hint">{{ t('admin.blocks.editor.untitled') }}</p>
        </div>
      </div>
    </template>

    <div v-if="blocks.length" class="be-slot be-slot--end">
      <button
        v-if="pickerIndex !== blocks.length"
        type="button"
        class="be-inserter"
        @click="openPicker(blocks.length)"
      >+ {{ t('admin.blocks.editor.addBlock') }}</button>
      <div v-else class="be-picker be-picker--grid">
        <button
          v-for="def in blockDefs"
          :key="def.type"
          type="button"
          class="be-type-card"
          @click="insertBlock(def, blocks.length)"
        >
          <span class="be-type-thumb"><BlockPreview :type="def.type" variant="thumb" /></span>
          <span class="be-type-name">{{ t(`admin.blocks.types.${def.labelKey}`) }}</span>
          <span class="be-type-desc">{{ t(`admin.blocks.typeDescs.${def.type}`) }}</span>
        </button>
        <button type="button" class="be-picker-cancel" @click="pickerIndex = null">{{ t('admin.blocks.editor.cancel') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { ArrowDown, ArrowUp, ChevronDown, ClipboardCopy, Copy, GripVertical, Trash2 } from 'lucide-vue-next'
import { notifySuccess } from '@/utils/notify'
import BlockField from '@/components/admin/blocks/BlockField.vue'
import BlockPreview from '@/components/admin/blocks/BlockPreview.vue'
import {
  blockDefs,
  createBlock,
  emptyPageContent,
  findBlockDef,
  type BlockDef,
  type FieldDef,
} from '@/lib/blocks/schema'
import { blockRecipes, type BlockRecipe } from '@/lib/blocks/recipes'

const props = defineProps<{
  modelValue: any
  /** 由外部（整页编辑器顶栏）统一控制当前编辑语言 */
  lang?: string
  selectedId?: string | null
}>()
const emit = defineEmits<{
  'update:modelValue': [value: any]
  'update:selectedId': [value: string | null]
  'update:lang': [value: string]
}>()
const { t } = useI18n()

const recipeOpen = ref(false)
/** 语言：面板里每个字段的多语言点也能直接切换，所以做成可写 ref 再回传给页面 */
const blockLang = ref(props.lang || 'zh-CN')
watch(() => props.lang, (v) => { if (v && v !== blockLang.value) blockLang.value = v })
watch(blockLang, (v) => { if (v !== props.lang) emit('update:lang', v) })
provide('blockActiveLang', blockLang)

const FIELD_GROUPS = ['content', 'look', 'link'] as const
const groupedFields = (type: string): Record<string, FieldDef[]> => {
  const groups: Record<string, FieldDef[]> = { content: [], look: [], link: [] }
  for (const field of fieldsOf(type)) {
    const key = field.group || 'content'
    ;(groups[key] ||= []).push(field)
  }
  return groups
}

/** 外观/链接默认收起，点开一次记住 */
const fieldsInGroup = (type: string, group: string): FieldDef[] => groupedFields(type)[group] ?? []

const groupOpen = ref<Record<string, boolean>>({})
const groupKey = (blockId: string, group: string) => `${blockId}:${group}`
const isGroupOpen = (blockId: string, group: string) => groupOpen.value[groupKey(blockId, group)] ?? false
const toggleGroup = (blockId: string, group: string) => {
  const key = groupKey(blockId, group)
  groupOpen.value = { ...groupOpen.value, [key]: !groupOpen.value[key] }
}

/** 选中的区块由外部控制（整页编辑器里两边联动） */
const openId = computed(() => props.selectedId ?? null)
const select = (id: string | null) => emit('update:selectedId', id)
/** 插入点：pickerIndex 指向「在第几个区块之前插入」 */
const pickerIndex = ref<number | null>(null)

const content = computed(() => {
  const raw = props.modelValue
  if (raw && typeof raw === 'object' && Array.isArray(raw.blocks)) return raw
  return emptyPageContent()
})

const blocks = computed<any[]>(() => content.value.blocks)

/* ---------- 撤销栈 ---------- */
const history = ref<any[][]>([])
const canUndo = computed(() => history.value.length > 0)
const clone = <T,>(v: T): T => JSON.parse(JSON.stringify(v))

const commit = (next: any[], record = true) => {
  if (record) {
    history.value.push(clone(blocks.value))
    if (history.value.length > 60) history.value.shift()
  }
  emit('update:modelValue', { version: 1, blocks: next })
}

const undo = () => {
  const prev = history.value.pop()
  if (!prev) return
  pickerIndex.value = null
  emit('update:modelValue', { version: 1, blocks: prev })
}

watch(
  () => props.selectedId,
  async (id) => {
    if (!id) return
    await nextTick()
    document.querySelector(`[data-block-id="${id}"]`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  },
)

const onKeydown = (e: KeyboardEvent) => {
  if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'z') return
  const el = document.activeElement as HTMLElement | null
  // 正在输入框里打字时交给浏览器自己撤销
  if (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)) return
  e.preventDefault()
  undo()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

/* ---------- 字段 ---------- */
const fieldsOf = (type: string): FieldDef[] => findBlockDef(type)?.fields || []

const typeLabel = (type: string) => {
  const def = findBlockDef(type)
  return def ? t(`admin.blocks.types.${def.labelKey}`) : type
}

/** 折叠状态下的摘要：按 title → subtitle → body 取当前语言 */
const summary = (block: any): string => {
  const def = findBlockDef(block.type)
  if (!def) return ''
  const pick = (v: any): string => {
    if (!v) return ''
    if (typeof v === 'string') return v
    return v[blockLang.value] || v['zh-CN'] || v['en-US'] || ''
  }
  for (const key of ['title', 'subtitle', 'body']) {
    const field = def.fields.find((f) => f.key === key)
    if (!field) continue
    const text = pick(block.props?.[key])
    if (text) return text
  }
  return ''
}

/* ---------- 增删改排 ---------- */
const openPicker = (index: number) => {
  pickerIndex.value = pickerIndex.value === index ? null : index
  recipeOpen.value = false
}

const insertBlock = async (def: BlockDef, index: number) => {
  const created = createBlock(def)
  const next = [...blocks.value]
  next.splice(index, 0, created)
  commit(next)
  pickerIndex.value = null
  select(created.id)
  await nextTick()
  const el = document.querySelector<HTMLElement>(`[data-block-id="${created.id}"]`)
  el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  el?.querySelector<HTMLInputElement>('.be-block-body input, .be-block-body textarea')?.focus({ preventScroll: true })
}

const remove = (index: number) => {
  commit(blocks.value.filter((_, i) => i !== index))
}

const move = (index: number, delta: number) => {
  const next = [...blocks.value]
  const target = index + delta
  if (target < 0 || target >= next.length) return
  ;[next[index], next[target]] = [next[target], next[index]]
  commit(next)
}

const duplicate = (index: number) => {
  const copy = clone(blocks.value[index])
  copy.id = `blk_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`
  const next = [...blocks.value]
  next.splice(index + 1, 0, copy)
  commit(next)
  select(copy.id)
}

const toggle = (id: string) => {
  select(openId.value === id ? null : id)
}

/* ---------- 跨文章复制/粘贴区块 ---------- */
const clipboard = ref<any | null>(null)

const copyBlock = (index: number) => {
  const block = blocks.value[index]
  if (!block) return
  clipboard.value = clone(block)
  try {
    navigator.clipboard?.writeText(JSON.stringify(block))
  } catch {
    /* 无剪贴板权限就只在面板内可用 */
  }
  notifySuccess(t('admin.blocks.editor.copied'))
}

const pasteBlock = () => {
  const source = clipboard.value
  if (!source) return
  const copy = clone(source)
  copy.id = `blk_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`
  const at = openId.value ? blocks.value.findIndex((b) => b.id === openId.value) + 1 : blocks.value.length
  const next = [...blocks.value]
  next.splice(at < 0 ? next.length : at, 0, copy)
  commit(next)
  select(copy.id)
}

/* ---------- 拖拽排序 ---------- */
const rootEl = ref<HTMLElement | null>(null)
const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)

/** 拖到面板上下边缘时自动滚动，长文章里也能一路拖到底 */
const scrollParentOf = (node: HTMLElement | null): HTMLElement | null => {
  let el = node?.parentElement || null
  while (el) {
    const style = getComputedStyle(el)
    if (/(auto|scroll)/.test(style.overflowY) && el.scrollHeight > el.clientHeight + 8) return el
    el = el.parentElement
  }
  return (document.scrollingElement as HTMLElement | null) || null
}

const onRootDragOver = (event: DragEvent) => {
  if (dragIndex.value === null) return
  const scroller = scrollParentOf(rootEl.value)
  if (!scroller) return
  const rect = scroller.getBoundingClientRect()
  const edge = 70
  const step = 14
  if (event.clientY < rect.top + edge) scroller.scrollTop -= step
  else if (event.clientY > rect.bottom - edge) scroller.scrollTop += step
}

const onDragStart = (index: number, e: DragEvent) => {
  dragIndex.value = index
  const block = (e.target as HTMLElement)?.closest('.be-block') as HTMLElement | null
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
    if (block) e.dataTransfer.setDragImage(block, 24, 24)
  }
}

const onDragOver = (index: number) => {
  if (dragIndex.value === null) return
  overIndex.value = index
}

const onDrop = (index: number) => {
  const from = dragIndex.value
  resetDrag()
  if (from === null || from === index) return
  const next = [...blocks.value]
  const [moved] = next.splice(from, 1)
  next.splice(index, 0, moved)
  commit(next)
}

const resetDrag = () => {
  dragIndex.value = null
  overIndex.value = null
}

defineExpose({ undo, canUndo, openPicker })

/* ---------- 版式 ---------- */
const applyRecipe = (recipe: BlockRecipe, replace = false) => {
  if (replace && blocks.value.length && !window.confirm(t('admin.blocks.editor.recipeReplaceConfirm'))) return
  const next = recipe.build()
  commit(next)
  select(next[0]?.id ?? null)
  recipeOpen.value = false
  pickerIndex.value = null
}
</script>

<style scoped>
.be {
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 12px;
  padding: 16px;
}
.be-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.be-head-left {
  min-width: 0;
}
.be-head-right {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
}
.be-title {
  font-size: 13px;
  font-weight: 600;
}
.be-hint {
  font-size: 12px;
  color: var(--muted-foreground, #9ca3af);
}
.be-head-actions {
  display: flex;
  gap: 8px;
}

/* ---------- 插入条 ---------- */
.be-slot {
  position: relative;
}
.be-inserter {
  display: block;
  width: 100%;
  height: 22px;
  margin: 2px 0;
  border: 0;
  border-radius: 6px;
  background: none;
  color: var(--muted-foreground, #9ca3af);
  font-size: 11px;
  line-height: 22px;
  cursor: pointer;
  opacity: 0;
  transition: opacity 120ms ease, background-color 120ms ease;
}
.be-slot:hover .be-inserter,
.be-inserter:focus-visible {
  opacity: 1;
  background: var(--muted, #f3f4f6);
}
.be-slot--end .be-inserter {
  opacity: 1;
  border: 1px dashed var(--border, #d1d5db);
}

/* ---------- 区块 ---------- */
.be-block {
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 10px;
  margin-bottom: 6px;
  overflow: hidden;
  transition: border-color 120ms ease, box-shadow 120ms ease;
}
.be-block.is-open {
  border-color: var(--foreground, #111827);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}
.be-block.is-dragging {
  opacity: 0.4;
}
.be-block.is-drop-target {
  box-shadow: inset 0 3px 0 0 var(--primary, #2563eb);
}
.be-block-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  background: var(--muted, #f9fafb);
  cursor: grab;
}
.be-block.is-open .be-block-head {
  background: var(--background, #fff);
}
.be-grip {
  display: inline-flex;
  flex: none;
  width: 14px;
  color: var(--muted-foreground, #cbd5e1);
  font-size: 12px;
  cursor: grab;
}
.be-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: none;
  text-align: left;
  cursor: pointer;
}
.be-index {
  flex: none;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--muted, #f3f4f6);
  color: var(--muted-foreground, #6b7280);
  font-size: 11px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.be-thumb {
  flex: none;
  width: 64px;
  height: 38px;
  padding: 3px;
  border-radius: 6px;
  background: #fff;
  border: 1px solid rgba(0, 0, 0, 0.06);
  overflow: hidden;
}
.be-thumb :deep(.bp) {
  max-height: 30px;
  overflow: hidden;
}
.be-block-title {
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.be-summary {
  margin-left: 8px;
  font-weight: 400;
  color: var(--muted-foreground, #9ca3af);
}
.be-chevron {
  display: inline-flex;
  flex: none;
  color: var(--muted-foreground, #9ca3af);
  transition: transform 200ms ease;
  transform: rotate(-90deg);
}
.be-chevron.is-open {
  transform: rotate(0deg);
}
.be-actions {
  display: flex;
  gap: 4px;
  flex: none;
}
.be-actions button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 6px;
  background: none;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.be-actions button:hover:not(:disabled) {
  color: var(--foreground, #111827);
}
.be-actions button:hover:not(:disabled) {
  border-color: var(--foreground, #111827);
}
.be-actions button:disabled {
  opacity: 0.35;
  cursor: default;
}
.be-actions .danger {
  color: #dc2626;
}
.be-block-body {
  padding: 14px;
}
.be-group + .be-group {
  margin-top: 6px;
}
.be-group-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  margin-bottom: 8px;
  padding: 6px 0;
  border: 0;
  border-top: 1px solid var(--border, #f1f5f9);
  background: none;
  font-size: 12px;
  font-weight: 500;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.be-group-chevron {
  width: 14px;
  height: 14px;
  transition: transform 160ms ease;
  transform: rotate(-90deg);
}
.be-group-chevron.is-open {
  transform: rotate(0deg);
}
.be-group-count {
  margin-left: auto;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.be-preview {
  margin-bottom: 14px;
}
.be-preview-hint {
  margin: 6px 0 0;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}

/* ---------- 语言 ---------- */
.be-langs {
  display: flex;
  gap: 4px;
  padding: 2px;
  border-radius: 8px;
  background: var(--muted, #f3f4f6);
}
.be-lang {
  min-width: 34px;
  height: 26px;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: none;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.be-lang.is-active {
  background: var(--background, #fff);
  color: var(--foreground, #111827);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}

/* ---------- 版式 / 空状态 ---------- */
.be-recipes {
  padding: 4px 0 8px;
}
.be-recipes-title {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
}
.be-recipes-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
}
.be-recipe {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 10px;
  background: none;
  text-align: left;
  cursor: pointer;
}
.be-recipe:hover {
  border-color: var(--foreground, #111827);
}
.be-recipe-name {
  font-size: 13px;
  font-weight: 600;
}
.be-recipe-desc {
  font-size: 11px;
  line-height: 1.4;
  color: var(--muted-foreground, #9ca3af);
}
.be-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 8px 0 12px;
  padding: 12px;
  border: 1px dashed var(--border, #d1d5db);
  border-radius: 10px;
}
.be-picker--grid {
  display: grid;
  /* 自适应容器宽度：在 430px 侧栏里是 2 列，在宽弹窗里自动变 4 列 */
  grid-template-columns: repeat(auto-fill, minmax(132px, 1fr));
  gap: 10px;
}
.be-picker-cancel {
  grid-column: 1 / -1;
  height: 28px;
  border: 0;
  border-radius: 6px;
  background: var(--muted, #f3f4f6);
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.be-type-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 10px;
  background: none;
  text-align: left;
  cursor: pointer;
  transition: border-color 160ms ease, box-shadow 160ms ease;
}
.be-type-card:hover {
  border-color: var(--foreground, #111827);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.06);
}
.be-type-thumb {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 58px;
  padding: 6px;
  border-radius: 8px;
  background: #fff;
  border: 1px solid rgba(0, 0, 0, 0.06);
  overflow: hidden;
}
.be-type-thumb :deep(.bp) {
  max-height: 46px;
  overflow: hidden;
}
.be-type-name {
  font-size: 12px;
  font-weight: 600;
}
.be-type-desc {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-size: 11px;
  line-height: 1.35;
  color: var(--muted-foreground, #9ca3af);
}
.be-pick-name {
  display: block;
  font-size: 13px;
  font-weight: 600;
}
.be-pick-desc {
  display: block;
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.be-pick {
  height: 30px;
  padding: 0 12px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 999px;
  background: none;
  font-size: 12px;
  cursor: pointer;
}
</style>
