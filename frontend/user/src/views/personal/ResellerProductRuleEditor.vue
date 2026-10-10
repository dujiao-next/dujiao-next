<template>
  <div class="space-y-4">
    <div class="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
      <div class="min-w-0">
        <div class="truncate text-sm font-bold text-foreground">{{ label }}</div>
        <div class="mt-1 flex flex-wrap items-center gap-1.5">
          <Badge variant="neutral" size="xs">{{ t('personalCenter.reseller.productSettings.basePrice') }} {{ basePrice }}</Badge>
          <Badge v-if="masterPrice" variant="neutral" size="xs">{{ t('personalCenter.reseller.productSettings.masterPrice') }} {{ masterPrice }}</Badge>
          <Badge :variant="invalid ? 'danger' : 'accent'" size="xs">{{ t('personalCenter.reseller.productSettings.effectivePrice') }} {{ effectivePrice || '-' }}</Badge>
          <span v-if="invalid && errorHint" class="text-xs font-medium text-destructive">{{ errorHint }}</span>
        </div>
      </div>
      <label class="inline-flex items-center gap-2 text-sm text-foreground">
        <Switch :model-value="modelValue.is_listed" @update:model-value="updateBoolean" />
        {{ t('personalCenter.reseller.productSettings.listed') }}
      </label>
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-[220px_minmax(0,1fr)]">
      <Select :model-value="modelValue.pricing_mode" @update:model-value="(v) => updateString('pricing_mode', v)">
        <SelectTrigger>
          <SelectValue :placeholder="t('personalCenter.reseller.productSettings.pricingMode')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="inherit">{{ t('personalCenter.reseller.productSettings.inherit') }}</SelectItem>
          <SelectItem value="markup_percent">{{ t('personalCenter.reseller.productSettings.markupPercent') }}</SelectItem>
          <SelectItem value="fixed_markup">{{ t('personalCenter.reseller.productSettings.fixedMarkup') }}</SelectItem>
          <SelectItem value="fixed_price">{{ t('personalCenter.reseller.productSettings.fixedPrice') }}</SelectItem>
        </SelectContent>
      </Select>
      <Input
        v-if="modelValue.pricing_mode === 'markup_percent'"
        :model-value="modelValue.markup_percent"
        type="text"
        inputmode="decimal"
        :placeholder="t('personalCenter.reseller.productSettings.markupPercentField')"
        @update:model-value="(v) => updateString('markup_percent', v)"
      />
      <Input
        v-else-if="modelValue.pricing_mode === 'fixed_markup'"
        :model-value="modelValue.fixed_markup_amount"
        type="text"
        inputmode="decimal"
        :placeholder="t('personalCenter.reseller.productSettings.fixedMarkupField')"
        @update:model-value="(v) => updateString('fixed_markup_amount', v)"
      />
      <Input
        v-else-if="modelValue.pricing_mode === 'fixed_price'"
        :model-value="modelValue.fixed_price_amount"
        type="text"
        inputmode="decimal"
        :placeholder="t('personalCenter.reseller.productSettings.fixedPriceField')"
        @update:model-value="(v) => updateString('fixed_price_amount', v)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { ResellerProductSettingPayloadItem } from '../../api/types'

const props = defineProps<{
  label: string
  basePrice: string
  masterPrice?: string
  effectivePrice?: string
  invalid?: boolean
  errorCode?: string
  modelValue: ResellerProductSettingPayloadItem
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: ResellerProductSettingPayloadItem): void
}>()

const { t } = useI18n()

const errorHint = computed(() => {
  if (!props.invalid) return ''
  if (props.errorCode === 'markup_exceeded') return t('personalCenter.reseller.productSettings.markupExceededHint')
  if (props.errorCode === 'price_below_master') return t('personalCenter.reseller.productSettings.priceBelowMasterHint')
  return t('personalCenter.reseller.productSettings.priceInvalidHint')
})

const updateValue = <K extends keyof ResellerProductSettingPayloadItem>(
  key: K,
  value: ResellerProductSettingPayloadItem[K],
) => {
  emit('update:modelValue', {
    ...props.modelValue,
    [key]: value,
  })
}

const updateString = (
  key: 'pricing_mode' | 'markup_percent' | 'fixed_markup_amount' | 'fixed_price_amount',
  value: unknown,
) => {
  updateValue(key, String(value ?? '').trim())
}

const updateBoolean = (value: boolean) => {
  updateValue('is_listed', value)
}
</script>
