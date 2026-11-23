<template>
  <nav :aria-label="t('checkoutSteps.label')" class="flex items-center gap-2 sm:gap-3">
    <template v-for="(step, idx) in steps" :key="step.key">
      <div class="flex items-center gap-2">
        <span
          class="grid h-6 w-6 shrink-0 place-items-center rounded-[7px] font-mono text-[11px] font-bold transition-colors"
          :class="stepCircleClass(idx)"
        >
          <Check v-if="idx < activeIndex" class="h-3.5 w-3.5" :stroke-width="2.5" aria-hidden="true" />
          <span v-else>{{ String(idx + 1).padStart(2, '0') }}</span>
        </span>
        <span
          class="whitespace-nowrap text-[13px] transition-colors"
          :class="idx === activeIndex ? 'font-semibold text-foreground' : (idx < activeIndex ? 'font-medium text-foreground/80' : 'text-muted-foreground')"
        >
          {{ step.label }}
        </span>
      </div>
      <span
        v-if="idx < steps.length - 1"
        class="h-px flex-1 min-w-4 transition-colors"
        :class="idx < activeIndex ? 'bg-primary/50' : 'bg-foreground/15'"
        aria-hidden="true"
      ></span>
    </template>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check } from 'lucide-vue-next'

type StepKey = 'cart' | 'checkout' | 'payment'

const props = withDefaults(defineProps<{
  currentStep: StepKey
  stepKeys?: StepKey[]
}>(), {
  stepKeys: () => ['cart', 'checkout', 'payment'],
})

const { t } = useI18n()

const labelMap: Record<StepKey, string> = {
  cart: 'cart.title',
  checkout: 'checkout.title',
  payment: 'payment.title',
}

const steps = computed(() =>
  props.stepKeys.map((key) => ({
    key,
    label: t(labelMap[key]),
  }))
)

const activeIndex = computed(() => {
  const idx = props.stepKeys.indexOf(props.currentStep)
  return idx >= 0 ? idx : 0
})

const stepCircleClass = (idx: number) => {
  if (idx === activeIndex.value) return 'bg-primary text-primary-foreground shadow-xs'
  if (idx < activeIndex.value) return 'border border-success/30 bg-success-soft text-success'
  return 'border bg-card text-muted-foreground'
}
</script>
