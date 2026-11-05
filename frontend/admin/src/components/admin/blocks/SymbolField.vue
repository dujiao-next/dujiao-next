<template>
  <div class="sf">
    <div class="sf-row">
      <span class="sf-preview" :style="{ fontSize: previewSize + 'px' }">
        <span v-if="modelValue">{{ modelValue }}</span>
        <span v-else class="sf-empty">?</span>
      </span>
      <Input
        :model-value="modelValue"
        class="sf-input"
        :placeholder="t('admin.symbolField.placeholder')"
        @update:model-value="(v) => emit('update:modelValue', String(v))"
      />
      <Button v-if="modelValue" type="button" size="sm" variant="outline" @click="emit('update:modelValue', '')">
        {{ t('admin.symbolField.clear') }}
      </Button>
    </div>
    <p v-if="showHint" class="sf-hint">{{ t('admin.symbolField.hint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

withDefaults(defineProps<{ modelValue: string; previewSize?: number; showHint?: boolean }>(), { previewSize: 28, showHint: true })
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const { t } = useI18n()
</script>

<style scoped>
.sf {
  min-width: 0;
}
.sf-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.sf-preview {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 48px;
  height: 40px;
  border: 1px solid var(--border, #e5e7eb);
  border-radius: 8px;
  background: var(--muted, #f9fafb);
  color: var(--foreground, #1d1d1f);
  /* SF Symbols 字符必须用苹果字体才出字形 */
  font-family: 'SF Pro', 'SF Pro Display', 'SF Pro SC', -apple-system, 'Apple Symbols', sans-serif;
  font-weight: 300;
  line-height: 1;
}
.sf-empty {
  font-family: inherit;
  font-size: 14px;
  color: var(--muted-foreground, #9ca3af);
}
.sf-input {
  flex: 1;
  min-width: 0;
  font-family: 'SF Pro', -apple-system, 'Apple Symbols', sans-serif;
}
.sf-hint {
  margin: 5px 0 0;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
</style>
