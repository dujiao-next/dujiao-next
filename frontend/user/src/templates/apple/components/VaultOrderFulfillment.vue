<template>
  <div class="grid gap-2">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h3 v-if="title" class="text-[15px] font-bold">{{ title }}</h3>
      <div class="flex flex-wrap gap-2">
        <Button
          v-if="isFulfillmentTruncated(fulfillment)"
          type="button"
          variant="outline"
          size="sm"
          class="rounded-full"
          :disabled="downloading"
          @click="emit('download', orderNo)"
        >
          <Download /> {{ downloading ? t('orderDetail.fulfillmentDownloading') : t('orderDetail.fulfillmentDownload') }}
        </Button>
        <Button
          v-if="fulfillment.status === 'delivered' && !isFulfillmentTruncated(fulfillment)"
          type="button"
          :variant="fulfillmentCopied ? 'default' : 'outline'"
          size="sm"
          class="rounded-full"
          :class="fulfillmentCopied ? 'bg-[color:var(--teal-strong)] text-white hover:bg-[color:var(--teal-strong)]/90' : ''"
          @click="handleCopyFulfillment(fulfillment)"
        >
          <component :is="fulfillmentCopied ? Check : Copy" /> {{ fulfillmentCopied ? t('orderDetail.fulfillmentCopied') : t('orderDetail.fulfillmentCopy') }}
        </Button>
      </div>
    </div>

    <div class="flex justify-between gap-3 text-[13px]"><span class="text-muted-foreground">{{ t('orderDetail.fulfillmentType') }}</span><span class="font-semibold text-foreground">{{ fulfillmentTypeLabelText(fulfillment.type) }}</span></div>
    <div class="flex justify-between gap-3 text-[13px]"><span class="text-muted-foreground">{{ t('orderDetail.fulfillmentStatus') }}</span><span class="font-semibold text-foreground">{{ fulfillmentStatusLabelText(fulfillment.status) }}</span></div>

    <template v-if="isFulfillmentTruncated(fulfillment)">
      <div class="text-xs text-muted-foreground">{{ t('orderDetail.fulfillmentTotalLines', { count: fulfillment.payload_line_count }) }}</div>
      <div class="rounded-sm bg-warning/10 px-2.5 py-2 text-xs font-semibold text-warning">{{ t('orderDetail.fulfillmentTruncatedHint') }}</div>
      <pre class="mt-1 max-h-[260px] overflow-y-auto whitespace-pre-wrap break-all rounded-sm border bg-secondary p-3 font-mono text-[12.5px] text-muted-foreground">{{ fulfillment.payload }}</pre>
    </template>
    <div v-else-if="fulfillmentDeliveryLines(fulfillment).length" class="mt-1 grid gap-0.5 rounded-sm border bg-secondary p-3 font-mono text-[12.5px] text-muted-foreground">
      <div v-for="(line, index) in fulfillmentDeliveryLines(fulfillment)" :key="index" class="whitespace-pre-wrap break-all">{{ line }}</div>
    </div>
    <pre v-else class="mt-1 whitespace-pre-wrap break-all rounded-sm border bg-secondary p-3 font-mono text-[12.5px] text-muted-foreground">{{ fulfillment.payload }}</pre>

    <div v-if="fulfillment.status === 'delivered' && instructionBlocks(items).length" class="mt-1.5 grid gap-2.5">
      <div v-for="(block, bi) in instructionBlocks(items)" :key="bi" class="rounded-md border border-primary/20 bg-primary/10 p-3.5">
        <div class="mb-2 flex items-center gap-2 text-[13px] font-bold text-primary"><Info class="h-4 w-4" /> {{ t('orderDetail.instructionsTitle') }}</div>
        <div class="prose prose-sm max-w-none dark:prose-invert prose-a:text-primary prose-img:rounded-sm" v-html="block.html"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, Check, Download, Info } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useOrderDisplayHelpers } from '../../../composables/useOrderDisplayHelpers'

defineProps<{
  title?: string
  fulfillment: any
  items?: any[]
  orderNo: string
  downloading: boolean
}>()

const emit = defineEmits<{
  (e: 'download', orderNo: string): void
}>()

const { t } = useI18n()

const {
  isFulfillmentTruncated, fulfillmentDeliveryLines, instructionBlocks,
  fulfillmentTypeLabelText, fulfillmentStatusLabelText, fulfillmentCopied, handleCopyFulfillment,
} = useOrderDisplayHelpers(ref<any>(null))
</script>
