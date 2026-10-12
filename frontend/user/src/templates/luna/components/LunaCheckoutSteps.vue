<template>
  <ol class="mb-6 flex flex-wrap items-center gap-2.5 p-0">
    <li
      v-for="(s, i) in steps"
      :key="s.key"
      class="flex items-center gap-2 text-[13px] font-semibold"
      :class="stateOf(s.key) === 'on' ? 'text-primary' : stateOf(s.key) === 'done' ? 'text-success' : 'text-muted-foreground'"
    >
      <span
        class="grid h-6 w-6 place-items-center rounded-full border text-xs"
        :class="stateOf(s.key) === 'on'
          ? 'border-primary bg-primary text-primary-foreground'
          : stateOf(s.key) === 'done'
            ? 'border-success bg-success text-primary-foreground'
            : 'border-border bg-secondary text-muted-foreground'"
      >
        <Check v-if="stateOf(s.key) === 'done'" class="h-3.5 w-3.5" />
        <template v-else>{{ s.num }}</template>
      </span>
      {{ s.label }}
      <span v-if="i < steps.length - 1" class="ml-1 h-0.5 w-[22px] bg-border"></span>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check } from 'lucide-vue-next'

type StepKey = 'cart' | 'checkout' | 'payment'

const props = defineProps<{
  current: StepKey
  skipCart?: boolean
}>()

const { t } = useI18n()

const labelOf = (key: StepKey) => {
  if (key === 'cart') return t('cart.title')
  if (key === 'checkout') return t('checkout.title')
  return t('payment.title')
}

const steps = computed(() => {
  const list: StepKey[] = props.skipCart ? ['checkout', 'payment'] : ['cart', 'checkout', 'payment']
  return list.map((key, idx) => ({ key, num: idx + 1, label: labelOf(key) }))
})

const currentIdx = computed(() => steps.value.findIndex((s) => s.key === props.current))

const stateOf = (key: StepKey): 'done' | 'on' | 'pending' => {
  const idx = steps.value.findIndex((s) => s.key === key)
  if (idx < currentIdx.value) return 'done'
  if (idx === currentIdx.value) return 'on'
  return 'pending'
}
</script>
