<template>
  <Transition
    enter-active-class="transition duration-300 ease-out"
    enter-from-class="translate-y-full opacity-0"
    enter-to-class="translate-y-0 opacity-100"
    leave-active-class="transition duration-200 ease-in"
    leave-from-class="translate-y-0 opacity-100"
    leave-to-class="translate-y-full opacity-0"
  >
    <div
      v-if="visible"
      class="theme-safe-bottom fixed bottom-0 left-0 right-0 z-40 border-t bg-card/95 shadow-[var(--shadow)] backdrop-blur-xl lg:hidden"
    >
      <div class="flex items-center gap-3 px-4 py-3">
        <!-- 价格 -->
        <div class="min-w-0 flex-1">
          <span v-if="showMemberPrice" class="block truncate text-xl font-extrabold tabular-nums text-[color:var(--gold-strong)]">{{ memberPriceDisplay }}</span>
          <span v-else-if="showSkuPromotionPrice" class="block truncate text-xl font-extrabold tabular-nums text-primary">{{ skuPromotionPriceDisplay }}</span>
          <span v-else-if="showSkuPrice" class="block truncate text-xl font-extrabold tabular-nums text-primary">{{ skuPriceDisplay }}</span>
          <span v-else-if="showProductPromotionPrice" class="block truncate text-xl font-extrabold tabular-nums text-primary">{{ productPromotionPriceDisplay }}</span>
          <span v-else class="block truncate text-xl font-extrabold tabular-nums text-primary">{{ productPriceDisplay }}</span>
        </div>
        <!-- 操作 -->
        <Button v-if="requiresLogin" size="lg" class="flex-none rounded-full font-bold" @click="$emit('goLogin')">{{ t('productDetail.loginToBuy') }}</Button>
        <template v-else>
          <Button size="lg" class="flex-none rounded-full font-bold" :disabled="!canPurchase" @click="$emit('buyNow')">
            <Zap /> {{ t('productDetail.buyNow') }}
          </Button>
        </template>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Zap } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

defineProps<{
  visible: boolean
  requiresLogin: boolean
  canPurchase: boolean
  showMemberPrice: boolean
  memberPriceDisplay: string
  showSkuPromotionPrice: boolean
  skuPromotionPriceDisplay: string
  showSkuPrice: boolean
  skuPriceDisplay: string
  showProductPromotionPrice: boolean
  productPromotionPriceDisplay: string
  productPriceDisplay: string
}>()

defineEmits<{
  buyNow: []
  goLogin: []
}>()
</script>
