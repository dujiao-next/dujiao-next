<template>
  <div class="bf">
    <label class="bf-label">
      {{ t(`admin.blocks.fields.${field.labelKey}`) }}
      <span v-if="field.hintKey" class="bf-hint">{{ t(`admin.blocks.hints.${field.hintKey}`) }}</span>
    </label>

    <Input
      v-if="field.kind === 'text' || field.kind === 'url'"
      :model-value="modelValue ?? ''"
      :placeholder="placeholder"
      @update:model-value="(v) => emit('update:modelValue', String(v))"
    />

    <Textarea
      v-else-if="field.kind === 'textarea'"
      :model-value="modelValue ?? ''"
      rows="5"
      @update:model-value="(v) => emit('update:modelValue', String(v))"
    />

    <div v-else-if="field.kind === 'i18n'" class="bf-langs">
      <div class="bf-lang-row">
        <textarea
          v-if="field.multiline"
          class="bf-multiline"
          rows="1"
          :ref="(el) => autoGrow(el as HTMLTextAreaElement | null)"
          :value="(modelValue || {})[activeLang.code] || ''"
          :placeholder="t('admin.blocks.editor.otherLangs')"
          @input="onMultilineInput"
          @keydown.enter.prevent="insertNewline"
        />
        <Input
          v-else
          :model-value="(modelValue || {})[activeLang.code] || ''"
          :placeholder="t('admin.blocks.editor.otherLangs')"
          @update:model-value="(v) => setI18n(activeLang.code, String(v))"
        />
        <span class="bf-dots" :title="t('admin.blocks.editor.langFilled')">
          <button
            v-for="l in langs"
            :key="l.code"
            type="button"
            class="bf-dot"
            :class="{ 'is-on': !!String((modelValue || {})[l.code] || '').trim(), 'is-active': l.code === activeLang.code }"
            :aria-label="l.code"
            @click="setLang(l.code)"
          >{{ l.short }}</button>
        </span>
      </div>
    </div>

    <select
      v-else-if="field.kind === 'select'"
      class="bf-select"
      :value="String(modelValue ?? '')"
      @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
    >
      <option v-for="opt in field.options" :key="opt.value" :value="opt.value">
        {{ t(`admin.blocks.fields.${opt.labelKey}`) }}
      </option>
    </select>

    <div v-else-if="field.kind === 'media'" class="bf-media">
      <div class="bf-seg">
        <button
          v-for="opt in mediaKinds"
          :key="opt.value"
          type="button"
          class="bf-seg-item"
          :class="{ 'is-active': media.kind === opt.value }"
          @click="setMedia('kind', opt.value)"
        >{{ t(`admin.blocks.fields.${opt.labelKey}`) }}</button>
      </div>
      <MediaPicker :model-value="media.src" scene="product" @update:model-value="(v) => setMedia('src', String(v || ''))" />
      <details class="bf-advanced" :open="!!(media.poster || media.mobileSrc)">
        <summary>{{ t('admin.blocks.fields.advancedMedia') }}</summary>
        <template v-if="media.kind === 'video'">
          <span class="bf-sub-label">{{ t('admin.blocks.fields.poster') }}</span>
          <MediaPicker :model-value="media.poster || ''" scene="product" @update:model-value="(v) => setMedia('poster', String(v || ''))" />
        </template>
        <span class="bf-sub-label">{{ t('admin.blocks.fields.mobileSrc') }}</span>
        <MediaPicker :model-value="media.mobileSrc || ''" scene="product" @update:model-value="(v) => setMedia('mobileSrc', String(v || ''))" />
      </details>
    </div>

    <Textarea
      v-else-if="field.kind === 'strings'"
      rows="3"
      :model-value="Array.isArray(modelValue) ? modelValue.join('\n') : ''"
      :placeholder="t('admin.blocks.fields.compareRowValuesHint')"
      @update:model-value="(v) => emit('update:modelValue', String(v).split('\n').map((x) => x.trim()).filter((x) => x !== ''))"
    />

    <div v-else-if="field.kind === 'list'" class="bf-list">
      <div
        v-for="(row, index) in listValue"
        :key="index"
        class="bf-item"
        :class="{ 'is-dragging': dragFrom === index, 'is-drop-target': dragOver === index && dragFrom !== null && dragFrom !== index }"
        @dragover.prevent="onRowDragOver(index)"
        @drop.prevent="onRowDrop(index)"
      >
        <div class="bf-item-head">
          <span
            class="bf-grip"
            role="button"
            tabindex="0"
            draggable="true"
            @dragstart="onRowDragStart(index, $event)"
            @dragend="resetRowDrag"
          ><GripVertical class="h-3.5 w-3.5" /></span>
          <button type="button" class="bf-item-toggle" @click="toggleRow(index)">
            <span class="bf-item-index">#{{ index + 1 }}</span>
            <span class="bf-item-summary">{{ rowSummary(row) }}</span>
            <span class="bf-item-chevron" :class="{ 'is-open': !isRowCollapsed(index) }"><ChevronDown class="h-4 w-4" /></span>
          </button>
          <button type="button" class="bf-link danger" @click="removeRow(index)">
            {{ t('admin.blocks.editor.remove') }}
          </button>
        </div>
        <div v-if="!isRowCollapsed(index)" class="bf-item-body">
          <BlockField
            v-for="sub in field.fields"
            :key="sub.key"
            :field="sub"
            :model-value="row[sub.key]"
            @update:model-value="(v) => (row[sub.key] = v)"
          />
        </div>
      </div>
      <button type="button" class="bf-add-row" @click="addRow">+ {{ t('admin.blocks.fields.' + field.labelKey) }}</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { inject, ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import { ChevronDown, GripVertical } from 'lucide-vue-next'
import type { FieldDef } from '@/lib/blocks/schema'

const props = defineProps<{
  field: FieldDef
  modelValue: any
}>()

const emit = defineEmits<{ 'update:modelValue': [value: any] }>()
const { t } = useI18n()

/* 当前编辑语言由 BlockEditor 提供，避免每个字段都渲染三行 */
const activeLangCode = inject<Ref<string>>('blockActiveLang', ref('zh-CN'))
const activeLang = computed(
  () => langs.find((l) => l.code === activeLangCode.value) || langs[0] || { code: 'zh-CN', short: '简', name: '简体中文' },
)


const langs = [
  { code: 'zh-CN', short: '简' },
  { code: 'zh-TW', short: '繁' },
  { code: 'en-US', short: 'EN' },
]

/** 点字段右侧的语言点直接切当前编辑语言 */
const setLang = (code: string) => {
  activeLangCode.value = code
}

const mediaKinds = [
  { value: 'image', labelKey: 'image' },
  { value: 'video', labelKey: 'video' },
]

const placeholder = computed(() => props.field.placeholderKey || '')

const media = computed<Record<string, any>>(() =>
  props.modelValue && typeof props.modelValue === 'object' ? props.modelValue : { kind: 'image', src: '' },
)

const setMedia = (key: string, value: string) => {
  emit('update:modelValue', { ...media.value, [key]: value })
}

const listValue = computed<any[]>(() => (Array.isArray(props.modelValue) ? props.modelValue : []))

/** 标题类字段：单行输入框按回车打不出换行，这里用自动增高的 textarea */
const autoGrow = (el: HTMLTextAreaElement | null) => {
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

const onMultilineInput = (event: Event) => {
  const el = event.target as HTMLTextAreaElement
  autoGrow(el)
  setI18n(activeLang.value.code, el.value)
}

const insertNewline = (event: KeyboardEvent) => {
  const el = event.target as HTMLTextAreaElement
  const start = el.selectionStart ?? el.value.length
  const end = el.selectionEnd ?? start
  const next = `${el.value.slice(0, start)}\n${el.value.slice(end)}`
  setI18n(activeLang.value.code, next)
  requestAnimationFrame(() => {
    el.value = next
    el.selectionStart = el.selectionEnd = start + 1
    autoGrow(el)
  })
}

const setI18n = (code: string, value: string) => {
  const next = { ...(props.modelValue || {}), [code]: value }
  emit('update:modelValue', next)
}

/* 行默认收起，只留一行摘要；几十个字段的卡片列表才不至于铺成几千像素 */
const expandedRows = ref<Set<number>>(new Set())
const isRowCollapsed = (index: number) => !expandedRows.value.has(index)
const toggleRow = (index: number) => {
  const next = new Set(expandedRows.value)
  if (next.has(index)) next.delete(index)
  else next.add(index)
  expandedRows.value = next
}

const rowSummary = (row: any): string => {
  const pick = (v: any): string => {
    if (!v) return ''
    if (typeof v === 'string') return v
    return v[activeLangCode.value] || v['zh-CN'] || v['en-US'] || ''
  }
  for (const key of ['title', 'label', 'text', 'name', 'value']) {
    if (!(props.field.fields || []).some((f) => f.key === key)) continue
    const text = pick(row?.[key])
    if (text) return text
  }
  return ''
}

const addRow = () => {
  const row = props.field.itemDefault ? props.field.itemDefault() : {}
  const next = [...listValue.value, row]
  expandedRows.value = new Set([...expandedRows.value, next.length - 1])
  emit('update:modelValue', next)
}

const removeRow = (index: number) => {
  emit('update:modelValue', listValue.value.filter((_, i) => i !== index))
}

/* 列表行拖拽换序（卡片顺序、参数顺序靠它，不然只能删了重加） */
const dragFrom = ref<number | null>(null)
const dragOver = ref<number | null>(null)

const onRowDragStart = (index: number, e: DragEvent) => {
  dragFrom.value = index
  const item = (e.target as HTMLElement)?.closest('.bf-item') as HTMLElement | null
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(index))
    if (item) e.dataTransfer.setDragImage(item, 20, 20)
  }
}

const onRowDragOver = (index: number) => {
  if (dragFrom.value === null) return
  dragOver.value = index
}

const resetRowDrag = () => {
  dragFrom.value = null
  dragOver.value = null
}

const onRowDrop = (index: number) => {
  const from = dragFrom.value
  resetRowDrag()
  if (from === null || from === index) return
  const next = [...listValue.value]
  const [moved] = next.splice(from, 1)
  next.splice(index, 0, moved)
  emit('update:modelValue', next)
}
</script>

<style scoped>
.bf-hint {
  display: block;
  margin-top: 3px;
  font-size: 11px;
  font-weight: 400;
  line-height: 1.4;
  color: var(--muted-foreground, #9ca3af);
}
.bf {
  margin-bottom: 16px;
}
.bf-label {
  display: block;
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--muted-foreground, #6b7280);
}
.bf-sub-label {
  display: block;
  margin: 8px 0 4px;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.bf-select {
  width: 100%;
  height: 36px;
  padding: 0 10px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 8px;
  background: var(--background, #fff);
  font-size: 13px;
}
.bf-langs {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.bf-lang-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.bf-multiline {
  flex: 1;
  min-width: 0;
  min-height: 36px;
  padding: 8px 10px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 8px;
  background: var(--background, #fff);
  font: inherit;
  font-size: 14px;
  line-height: 1.4;
  color: var(--foreground, #111827);
  resize: none;
  overflow: hidden;
}
.bf-multiline:focus {
  outline: none;
  border-color: var(--foreground, #111827);
}
.bf-lang {
  flex: none;
  width: 26px;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.bf-media {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.bf-seg {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: var(--muted, #f3f4f6);
  align-self: flex-start;
}
.bf-seg-item {
  height: 24px;
  padding: 0 10px;
  border: 0;
  border-radius: 6px;
  background: none;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.bf-seg-item.is-active {
  background: var(--background, #fff);
  color: var(--foreground, #111827);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}
.bf-advanced {
  border-top: 1px solid var(--border, #f1f5f9);
  padding-top: 6px;
}
.bf-advanced summary {
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
  cursor: pointer;
}
.bf-advanced[open] summary {
  margin-bottom: 6px;
}
.bf-dots {
  display: inline-flex;
  flex: none;
  gap: 2px;
}
.bf-dot {
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: none;
  font-size: 10px;
  color: var(--muted-foreground, #cbd5e1);
  cursor: pointer;
}
.bf-dot.is-on {
  color: var(--foreground, #111827);
  background: var(--muted, #f3f4f6);
}
.bf-dot.is-active {
  outline: 1px solid var(--primary, #2563eb);
}
.bf-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.bf-item {
  padding: 12px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 10px;
  background: var(--background, #fff);
}
.bf-item.is-dragging {
  opacity: 0.4;
}
.bf-item.is-drop-target {
  box-shadow: inset 0 3px 0 0 var(--primary, #2563eb);
}
.bf-grip {
  display: inline-flex;
  width: 14px;
  color: var(--muted-foreground, #cbd5e1);
  font-size: 12px;
  cursor: grab;
}
.bf-item-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  padding: 0;
  border: 0;
  background: none;
  text-align: left;
  cursor: pointer;
}
.bf-item-summary {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  color: var(--foreground, #111827);
}
.bf-item-chevron {
  display: inline-flex;
  margin-left: auto;
  color: var(--muted-foreground, #9ca3af);
  transform: rotate(-90deg);
  transition: transform 160ms ease;
}
.bf-item-chevron.is-open {
  transform: rotate(0deg);
}
.bf-item-body {
  margin-top: 10px;
}
.bf-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.bf-item-index {
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.bf-add-row {
  height: 34px;
  border: 1px dashed var(--border, #d1d5db);
  border-radius: 8px;
  background: none;
  font-size: 13px;
  cursor: pointer;
}
.bf-link {
  border: 0;
  background: none;
  font-size: 12px;
  cursor: pointer;
  color: var(--primary, #2563eb);
}
.bf-link.danger {
  color: #dc2626;
}
</style>
