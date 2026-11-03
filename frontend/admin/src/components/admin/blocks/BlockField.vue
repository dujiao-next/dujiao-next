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
        <span class="bf-lang">{{ activeLang.short }}</span>
        <Input
          :model-value="(modelValue || {})[activeLang.code] || ''"
          :placeholder="t('admin.blocks.editor.otherLangs')"
          @update:model-value="(v) => setI18n(activeLang.code, String(v))"
        />
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
      <select :value="media.kind" class="bf-select" @change="setMedia('kind', ($event.target as HTMLSelectElement).value)">
        <option value="image">{{ t('admin.blocks.fields.image') }}</option>
        <option value="video">{{ t('admin.blocks.fields.video') }}</option>
      </select>
      <MediaPicker :model-value="media.src" scene="product" @update:model-value="(v) => setMedia('src', String(v || ''))" />
      <template v-if="media.kind === 'video'">
        <span class="bf-sub-label">{{ t('admin.blocks.fields.poster') }}</span>
        <MediaPicker :model-value="media.poster || ''" scene="product" @update:model-value="(v) => setMedia('poster', String(v || ''))" />
      </template>
      <span class="bf-sub-label">{{ t('admin.blocks.fields.mobileSrc') }}</span>
      <MediaPicker :model-value="media.mobileSrc || ''" scene="product" @update:model-value="(v) => setMedia('mobileSrc', String(v || ''))" />
    </div>

    <div v-else-if="field.kind === 'list'" class="bf-list">
      <div v-for="(row, index) in listValue" :key="index" class="bf-item">
        <div class="bf-item-head">
          <span class="bf-item-index">#{{ index + 1 }}</span>
          <button type="button" class="bf-link danger" @click="removeRow(index)">
            {{ t('admin.blocks.editor.remove') }}
          </button>
        </div>
        <BlockField
          v-for="sub in field.fields"
          :key="sub.key"
          :field="sub"
          :model-value="row[sub.key]"
          @update:model-value="(v) => (row[sub.key] = v)"
        />
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

const placeholder = computed(() => props.field.placeholderKey || '')

const media = computed<Record<string, any>>(() =>
  props.modelValue && typeof props.modelValue === 'object' ? props.modelValue : { kind: 'image', src: '' },
)

const setMedia = (key: string, value: string) => {
  emit('update:modelValue', { ...media.value, [key]: value })
}

const listValue = computed<any[]>(() => (Array.isArray(props.modelValue) ? props.modelValue : []))

const setI18n = (code: string, value: string) => {
  const next = { ...(props.modelValue || {}), [code]: value }
  emit('update:modelValue', next)
}

const addRow = () => {
  const row = props.field.itemDefault ? props.field.itemDefault() : {}
  emit('update:modelValue', [...listValue.value, row])
}

const removeRow = (index: number) => {
  emit('update:modelValue', listValue.value.filter((_, i) => i !== index))
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
.bf-lang {
  flex: none;
  width: 26px;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.bf-media {
  display: flex;
  flex-direction: column;
  gap: 6px;
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
