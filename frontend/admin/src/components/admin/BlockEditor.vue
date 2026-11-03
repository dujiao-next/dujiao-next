<template>
  <div class="be">
    <div class="be-head">
      <div>
        <div class="be-title">{{ t('admin.blocks.editor.picker') }}</div>
        <div class="be-hint">{{ t('admin.blocks.editor.addBlock') }} · {{ blocks.length }}</div>
      </div>
      <div class="be-langs">
        <button
          v-for="lang in blockLangs"
          :key="lang.code"
          type="button"
          class="be-lang"
          :class="{ 'is-active': blockLang === lang.code }"
          @click="blockLang = lang.code"
        >{{ lang.short }}</button>
      </div>
      <div class="be-head-actions">
        <Button v-if="blocks.length" size="sm" variant="outline" type="button" @click="recipeOpen = !recipeOpen">
          {{ t('admin.blocks.editor.useRecipe') }}
        </Button>
        <Button size="sm" type="button" @click="pickerOpen = !pickerOpen">
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

    <div v-if="pickerOpen" class="be-picker be-picker--grid">
      <button
        v-for="def in blockDefs"
        :key="def.type"
        type="button"
        class="be-type-card"
        @click="addBlock(def)"
      >
        <span class="be-type-thumb"><BlockPreview :type="def.type" variant="thumb" /></span>
        <span class="be-type-name">{{ t(`admin.blocks.types.${def.labelKey}`) }}</span>
        <span class="be-type-desc">{{ t(`admin.blocks.typeDescs.${def.type}`) }}</span>
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

    <div v-for="(block, index) in blocks" :key="block.id" class="be-block">
      <div class="be-block-head">
        <button type="button" class="be-toggle" @click="toggle(block.id)">
          <span class="be-index">{{ index + 1 }}</span>
          <span class="be-thumb"><BlockPreview :type="block.type" :props="block.props" variant="thumb" /></span>
          <span class="be-block-title">
            {{ typeLabel(block.type) }}
            <span v-if="summary(block)" class="be-summary">{{ summary(block) }}</span>
          </span>
          <span class="be-chevron" :class="{ 'is-open': collapsed[block.id] !== true }">⌄</span>
        </button>
        <div class="be-actions">
          <button type="button" :title="t('admin.blocks.editor.moveUp')" :disabled="index === 0" @click="move(index, -1)">↑</button>
          <button type="button" :title="t('admin.blocks.editor.moveDown')" :disabled="index === blocks.length - 1" @click="move(index, 1)">↓</button>
          <button type="button" :title="t('admin.blocks.editor.duplicate')" @click="duplicate(index)">⧉</button>
          <button type="button" class="danger" :title="t('admin.blocks.editor.remove')" @click="remove(index)">✕</button>
        </div>
      </div>

      <div v-if="collapsed[block.id] !== true" class="be-block-body">
        <div class="be-preview">
          <BlockPreview :type="block.type" :props="block.props" variant="full" />
          <p class="be-preview-hint">{{ t('admin.blocks.editor.previewHint') }}</p>
        </div>
        <BlockField
          v-for="field in fieldsOf(block.type)"
          :key="field.key"
          :field="field"
          :model-value="block.props[field.key]"
          @update:model-value="(v) => (block.props[field.key] = v)"
        />
        <p v-if="!fieldsOf(block.type).length" class="be-hint">{{ t('admin.blocks.editor.untitled') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, provide, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
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

const props = defineProps<{ modelValue: any }>()
const emit = defineEmits<{ 'update:modelValue': [value: any] }>()
const { t } = useI18n()

const pickerOpen = ref(false)
const blockLangs = [
  { code: 'zh-CN', short: '简' },
  { code: 'zh-TW', short: '繁' },
  { code: 'en-US', short: 'EN' },
]
const blockLang = ref('zh-CN')
provide('blockActiveLang', blockLang)
const recipeOpen = ref(false)
const collapsed = reactive<Record<string, boolean>>({})

const content = computed(() => {
  const raw = props.modelValue
  if (raw && typeof raw === 'object' && Array.isArray(raw.blocks)) return raw
  return emptyPageContent()
})

const blocks = computed<any[]>(() => content.value.blocks)

const fieldsOf = (type: string): FieldDef[] => findBlockDef(type)?.fields || []

const typeLabel = (type: string) => {
  const def = findBlockDef(type)
  return def ? t(`admin.blocks.types.${def.labelKey}`) : type
}

/** 折叠状态下的摘要：取第一个有内容的 i18n 文本 */
const summary = (block: any): string => {
  const def = findBlockDef(block.type)
  const titleField = def?.fields.find((f) => f.key === 'title')
  if (!titleField) return ''
  const value = block.props?.title
  if (!value) return ''
  if (typeof value === 'string') return value
  return value['zh-CN'] || value['en-US'] || ''
}

const commit = (nextBlocks: any[]) => {
  emit('update:modelValue', { version: 1, blocks: nextBlocks })
}

const addBlock = (def: BlockDef) => {
  const created = createBlock(def)
  commit([...blocks.value, created])
  collapsed[created.id] = false
  pickerOpen.value = false
}

/** 套用版式模板：空页直接铺，已有内容时先确认覆盖 */
const applyRecipe = (recipe: BlockRecipe, replace = false) => {
  if (replace && !window.confirm(t('admin.blocks.editor.recipeReplaceConfirm'))) return
  const next = recipe.build()
  commit(next)
  // 套用后只展开第一个区块，其余收起，避免一屏几十个字段
  next.forEach((blk: any, i: number) => { collapsed[blk.id] = i !== 0 })
  recipeOpen.value = false
  pickerOpen.value = false
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
  const copy = JSON.parse(JSON.stringify(blocks.value[index]))
  copy.id = `blk_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`
  const next = [...blocks.value]
  next.splice(index + 1, 0, copy)
  commit(next)
}

const toggle = (id: string) => {
  collapsed[id] = collapsed[id] !== true
}
</script>

<style scoped>
.be-picker--grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 12px;
  margin-bottom: 12px;
}
@media (min-width: 900px) {
  .be-picker--grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
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
.be-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
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
.be-chevron {
  flex: none;
  margin-left: auto;
  font-size: 16px;
  color: var(--muted-foreground, #9ca3af);
  transition: transform 200ms ease;
  transform: rotate(-90deg);
}
.be-chevron.is-open {
  transform: rotate(0deg);
}
.be-preview {
  margin-bottom: 14px;
}
.be-preview-hint {
  margin: 6px 0 0;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
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
.be-head-actions {
  display: flex;
  gap: 8px;
}
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
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
@media (min-width: 900px) {
  .be-recipes-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
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
.be {
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 12px;
  padding: 16px;
}
.be-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.be-title {
  font-size: 13px;
  font-weight: 600;
}
.be-hint {
  font-size: 12px;
  color: var(--muted-foreground, #9ca3af);
}
.be-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  padding: 12px;
  border: 1px dashed var(--border, #d1d5db);
  border-radius: 10px;
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
.be-empty {
  padding: 24px;
  text-align: center;
  font-size: 13px;
  color: var(--muted-foreground, #9ca3af);
}
.be-block {
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 10px;
  margin-bottom: 10px;
  overflow: hidden;
}
.be-block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 12px;
  background: var(--muted, #f9fafb);
}
.be-toggle {
  display: flex;
  min-width: 0;
  flex: 1;
  border: 0;
  background: none;
  text-align: left;
  cursor: pointer;
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
.be-actions {
  display: flex;
  gap: 4px;
  flex: none;
}
.be-actions button {
  width: 26px;
  height: 26px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 6px;
  background: none;
  font-size: 12px;
  cursor: pointer;
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
</style>
