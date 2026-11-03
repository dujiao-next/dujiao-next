<template>
  <ol class="steps">
    <li
      v-for="(s, i) in steps"
      :key="s.key"
      class="steps-item"
      :class="`is-${stateOf(s.key)}`"
    >
      <span class="steps-num">
        <Check v-if="stateOf(s.key) === 'done'" class="h-3 w-3" />
        <template v-else>{{ s.num }}</template>
      </span>
      <span class="steps-label">{{ s.label }}</span>
      <span v-if="i < steps.length - 1" class="steps-line"></span>
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

<style scoped>
.steps {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.steps-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--st-text-3);
}
.steps-num {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  background: var(--st-bg-alt);
  font-size: 11px;
  font-weight: 600;
  color: var(--st-text-3);
}
.steps-item.is-on {
  color: var(--st-text);
}
.steps-item.is-on .steps-num {
  background: var(--st-text);
  color: var(--st-bg);
}
.steps-item.is-done {
  color: var(--st-text-2);
}
.steps-item.is-done .steps-num {
  background: var(--st-green);
  color: #fff;
}
.steps-label {
  font-weight: 600;
}
.steps-line {
  width: 20px;
  height: 1px;
  margin-inline: 2px;
  background: color-mix(in srgb, var(--st-text) 14%, transparent);
}
</style>
