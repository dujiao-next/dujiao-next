<template>
  <!--
    Nova 步骤条
    与 ember 版的差异：参考站点用「细线 + 全大写小字」表达流程，而不是实心数字球。
    已完成步骤用对勾 + 强调色文字，当前步骤用实心强调色圆点 + 加粗文字。
  -->
  <ol class="mb-7 flex flex-wrap items-center gap-x-3 gap-y-2 p-0">
    <li
      v-for="(s, i) in steps"
      :key="s.key"
      class="flex items-center gap-2.5"
    >
      <span
        class="grid h-[22px] w-[22px] flex-none place-items-center rounded-full border text-[11px] transition-colors duration-200"
        :class="stateOf(s.key) === 'on'
          ? 'border-[color:var(--accent)] bg-[color:var(--accent)] text-[color:var(--on-accent)]'
          : stateOf(s.key) === 'done'
            ? 'border-[color:var(--accent)] bg-[color:var(--accent-soft)] text-[color:var(--accent-strong)]'
            : 'border-border bg-transparent text-[color:var(--ink-4)]'"
      >
        <Check v-if="stateOf(s.key) === 'done'" class="h-3 w-3" />
        <template v-else>{{ s.num }}</template>
      </span>

      <span
        class="text-[13px] transition-colors duration-200"
        :class="stateOf(s.key) === 'on'
          ? 'font-semibold text-foreground'
          : stateOf(s.key) === 'done'
            ? 'text-[color:var(--ink-2)]'
            : 'text-[color:var(--ink-4)]'"
      >{{ s.label }}</span>

      <span
        v-if="i < steps.length - 1"
        class="ml-1 hidden h-px w-[28px] bg-border sm:block"
        aria-hidden="true"
      ></span>
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
  if (key === 'cart') return t('nova.steps.cart')
  if (key === 'checkout') return t('nova.steps.confirm')
  return t('nova.steps.pay')
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
