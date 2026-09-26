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
      class="ember-glass fixed bottom-0 left-0 right-0 z-40 border-t border-border shadow-[var(--shadow-lg)] lg:hidden"
      style="padding-bottom: env(safe-area-inset-bottom, 0px)"
    >
      <div class="flex items-center gap-3 px-4 py-3">
        <div class="min-w-0 flex-1">
          <span v-if="showMemberPrice" class="block truncate text-[19px] font-semibold tabular-nums tracking-[-0.03em] text-[color:var(--success-strong)]">
            {{ memberPriceDisplay }}
          </span>
          <span v-else-if="showSkuPromotionPrice" class="block truncate text-[19px] font-semibold tabular-nums tracking-[-0.03em] text-[color:var(--accent-strong)]">
            {{ skuPromotionPriceDisplay }}
          </span>
          <span v-else-if="showSkuPrice" class="block truncate text-[19px] font-semibold tabular-nums tracking-[-0.03em] text-[color:var(--accent-strong)]">
            {{ skuPriceDisplay }}
          </span>
          <span v-else-if="showProductPromotionPrice" class="block truncate text-[19px] font-semibold tabular-nums tracking-[-0.03em] text-[color:var(--accent-strong)]">
            {{ productPromotionPriceDisplay }}
          </span>
          <span v-else class="block truncate text-[19px] font-semibold tabular-nums tracking-[-0.03em] text-foreground">
            {{ productPriceDisplay }}
          </span>
        </div>

        <Button v-if="requiresLogin" size="lg" class="flex-none rounded-full text-[14px] font-medium" @click="$emit('goLogin')">
          {{ t('productDetail.loginToBuy') }}
        </Button>
        <template v-else>
          <Button
            variant="outline"
            size="lg"
            class="flex-none rounded-full border-[color:var(--border-strong)] text-[14px] font-medium"
            :disabled="!canPurchase"
            @click="$emit('addToCart')"
          >
            <ShoppingCart class="h-4 w-4" />
          </Button>
          <Button size="lg" class="flex-none rounded-full px-5 text-[14px] font-medium" :disabled="!canPurchase" @click="$emit('buyNow')">
            {{ t('productDetail.buyNow') }}
          </Button>
        </template>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ShoppingCart } from 'lucide-vue-next'
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
  addToCart: []
  buyNow: []
  goLogin: []
}>()
</script>
