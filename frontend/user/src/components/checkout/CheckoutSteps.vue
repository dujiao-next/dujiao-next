<template>
  <ol
    class="flex list-none items-center"
    :class="plain ? '' : 'rounded-2xl border bg-card/80 p-4 backdrop-blur'"
    :aria-label="t('checkoutSteps.label')"
  >
    <template v-for="(step, idx) in steps" :key="step.key">
      <li
        class="flex items-center gap-2"
        :class="idx === 0 ? '' : 'flex-1'"
        :aria-current="step.status === 'current' ? 'step' : undefined"
      >
        <div
          v-if="idx > 0"
          aria-hidden="true"
          class="flex-1 h-0.5 rounded-full transition-colors"
          :class="step.status === 'upcoming' ? 'bg-muted' : plain ? 'bg-foreground' : 'bg-primary'"
        ></div>
        <div class="flex items-center gap-2 shrink-0">
          <span
            class="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold border-2 transition-colors"
            :class="step.status === 'upcoming'
              ? 'border-border text-muted-foreground'
              : plain ? 'border-transparent bg-foreground text-background' : 'border-transparent bg-primary text-primary-foreground'"
          >
            <Check v-if="step.status === 'done'" class="w-3.5 h-3.5" :stroke-width="3" />
            <span v-else>{{ idx + 1 }}</span>
          </span>
          <span
            class="text-sm font-medium hidden sm:inline"
            :class="step.status !== 'upcoming' ? 'text-foreground' : 'text-muted-foreground'"
          >
            {{ step.label }}
          </span>
        </div>
      </li>
    </template>
  </ol>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check } from 'lucide-vue-next'

type StepKey = 'cart' | 'checkout' | 'payment'
type StepStatus = 'done' | 'current' | 'upcoming'

const props = withDefaults(
  defineProps<{
    currentStep: StepKey
    stepKeys?: StepKey[]
    // 默认模板用：去掉外框，完成态用墨色而非主题色
    plain?: boolean
  }>(),
  {
    stepKeys: () => ['cart', 'checkout', 'payment'],
  },
)

const { t } = useI18n()

const steps = computed(() => {
  const activeIdx = props.stepKeys.indexOf(props.currentStep)
  if (activeIdx < 0 && import.meta.env.DEV) {
    const brandWarn = (globalThis as any).console?.warn?.bind(console)
    brandWarn?.(`[CheckoutSteps] currentStep "${props.currentStep}" not found in stepKeys`, props.stepKeys)
  }
  return props.stepKeys.map((key, idx) => {
    const status: StepStatus = activeIdx < 0
      ? 'upcoming'
      : idx < activeIdx
        ? 'done'
        : idx === activeIdx
          ? 'current'
          : 'upcoming'
    return { key, label: t(`${key}.title`), status }
  })
})
</script>
