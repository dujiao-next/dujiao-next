<template>
  <div
    v-if="manualFormProducts.length"
    class="rounded-2xl border bg-card text-card-foreground p-6"
  >
    <h2 class="mb-2 text-lg font-bold text-foreground">{{ t('checkout.manualFormTitle') }}</h2>
    <p class="mb-4 text-xs text-muted-foreground">{{ t('checkout.manualFormTip') }}</p>
    <div class="space-y-5">
      <div
        v-for="manualItem in manualFormProducts"
        :key="manualItem.itemKey"
        class="rounded-2xl bg-secondary/60 p-4"
      >
        <h3 class="mb-3 text-sm font-semibold text-foreground">{{ manualItemTitle(manualItem) }}</h3>
        <div class="grid grid-cols-1 gap-x-4 gap-y-5 md:grid-cols-2">
          <div v-for="field in manualItem.fields" :key="`${manualItem.itemKey}-${field.key}`" class="space-y-1.5">
            <label class="block text-[13px] font-medium text-muted-foreground">
              {{ getManualFieldLabel(field) }}
              <span v-if="field.required" class="ml-1 text-destructive">*</span>
            </label>

            <Textarea
              v-if="field.type === 'textarea'"
              :model-value="getFieldValue(manualItem.itemKey, field.key)"
              @update:model-value="updateFieldValue(manualItem.itemKey, field.key, $event)"
              rows="3"
              class="bg-card"
              :placeholder="getManualFieldPlaceholder(field)"
            />

            <Select
              v-else-if="field.type === 'select'"
              :model-value="getFieldValue(manualItem.itemKey, field.key)"
              @update:model-value="updateFieldValue(manualItem.itemKey, field.key, $event)"
            >
              <SelectTrigger class="h-10 w-full bg-card">
                <SelectValue :placeholder="t('checkout.manualFormSelectPlaceholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in field.options" :key="option" :value="option">{{ option }}</SelectItem>
              </SelectContent>
            </Select>

            <div v-else-if="field.type === 'radio'" class="flex flex-wrap gap-2">
              <label v-for="option in field.options" :key="option" class="inline-flex h-9 cursor-pointer items-center gap-2 rounded-full bg-card px-3.5 text-sm text-foreground">
                <input
                  :checked="getFieldValue(manualItem.itemKey, field.key) === option"
                  @change="updateFieldValue(manualItem.itemKey, field.key, option)"
                  type="radio"
                  :name="`manual-radio-${manualItem.itemKey}-${field.key}`"
                  :value="option"
                  class="h-4 w-4 shrink-0 appearance-none rounded-full border border-input bg-card transition-[border-width] checked:border-[5px] checked:border-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary [.vault-tokens_&]:checked:border-primary"
                />
                <span>{{ option }}</span>
              </label>
            </div>

            <div v-else-if="field.type === 'checkbox'" class="flex flex-wrap gap-2">
              <label v-for="option in field.options" :key="option" class="inline-flex h-9 cursor-pointer items-center gap-2 rounded-full bg-card px-3.5 text-sm text-foreground">
                <Checkbox
                  :model-value="isCheckboxChecked(manualItem.itemKey, field.key, option)"
                  @update:model-value="toggleCheckboxValue(manualItem.itemKey, field.key, option, $event === true)"
                />
                <span>{{ option }}</span>
              </label>
            </div>

            <Input
              v-else
              :model-value="getFieldValue(manualItem.itemKey, field.key)"
              @update:model-value="updateFieldValue(manualItem.itemKey, field.key, $event)"
              :type="field.type === 'number' ? 'number' : field.type === 'email' ? 'email' : field.type === 'phone' ? 'tel' : 'text'"
              :placeholder="getManualFieldPlaceholder(field)"
              class="h-10 bg-card"
            />

            <p
              v-if="submitAttempted && manualFieldError(manualItem.itemKey, field.key)"
              class="text-xs text-destructive"
            >
              {{ manualFieldError(manualItem.itemKey, field.key) }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useLocalized } from '../../composables/useProduct'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

interface ManualFormField {
  key: string
  type: string
  required: boolean
  label?: Record<string, string>
  placeholder?: Record<string, string>
  regex?: string
  min?: number
  max?: number
  max_len?: number
  options: string[]
}

interface ManualFormProduct {
  itemKey: string
  productId: number
  title: any
  fields: ManualFormField[]
  skuCount: number
}

const props = defineProps<{
  manualFormProducts: ManualFormProduct[]
  modelValue: Record<string, Record<string, any>>
  submitAttempted: boolean
  getManualFieldLabel: (field: ManualFormField) => string
  getManualFieldPlaceholder: (field: ManualFormField) => string
  manualFieldError: (itemKey: string, fieldKey: string) => string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, Record<string, any>>): void
}>()

const { t } = useI18n()
const { getLocalizedText } = useLocalized()

const manualItemTitle = (manualItem: ManualFormProduct) => {
  const productTitle = getLocalizedText(manualItem.title)
  if (manualItem.skuCount <= 1) return productTitle
  return `${productTitle} (${t('checkout.manualFormAppliesToSkuCount', { count: manualItem.skuCount })})`
}

const getFieldValue = (itemKey: string, fieldKey: string) => {
  return props.modelValue[itemKey]?.[fieldKey] ?? ''
}

const updateFieldValue = (itemKey: string, fieldKey: string, value: any) => {
  const updated = { ...props.modelValue }
  if (!updated[itemKey]) {
    updated[itemKey] = {}
  }
  updated[itemKey] = { ...updated[itemKey], [fieldKey]: value }
  emit('update:modelValue', updated)
}

const isCheckboxChecked = (itemKey: string, fieldKey: string, option: string) => {
  const value = props.modelValue[itemKey]?.[fieldKey]
  return Array.isArray(value) && value.includes(option)
}

const toggleCheckboxValue = (itemKey: string, fieldKey: string, option: string, checked: boolean) => {
  const current = props.modelValue[itemKey]?.[fieldKey]
  const list = Array.isArray(current) ? [...current] : []
  if (checked) {
    if (!list.includes(option)) list.push(option)
  } else {
    const idx = list.indexOf(option)
    if (idx !== -1) list.splice(idx, 1)
  }
  updateFieldValue(itemKey, fieldKey, list)
}
</script>
