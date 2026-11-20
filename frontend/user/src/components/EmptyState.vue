<template>
  <div
    class="text-center theme-slide-up rounded-[22px] border border-dashed"
    :class="[
      sizeClass,
    ]"
  >
    <div class="flex justify-center" :class="iconWrapperClass">
      <slot name="icon">
        <component :is="resolvedIcon" :class="iconClass" aria-hidden="true" />
      </slot>
    </div>

    <p v-if="title" class="font-medium text-foreground" :class="titleClass">{{ title }}</p>
    <p v-if="description" class="mt-2 mx-auto text-muted-foreground" :class="descriptionClass">
      {{ description }}
    </p>

    <div v-if="hasAction" class="mt-6 flex flex-wrap justify-center gap-3">
      <slot name="action">
        <Button v-if="actionTo" as-child>
          <router-link :to="actionTo">{{ actionLabel }}</router-link>
        </Button>
        <Button
          v-else-if="actionLabel"
          type="button"
          :disabled="actionDisabled"
          @click="$emit('action')"
        >
          {{ actionLabel }}
        </Button>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, useSlots } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { AlertCircle, ClipboardList, Inbox, Package, Search, ShoppingCart } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

type IconName = 'package' | 'cart' | 'order' | 'search' | 'inbox' | 'alert'
type Size = 'sm' | 'md' | 'lg'

const props = withDefaults(
  defineProps<{
    icon?: IconName
    title?: string
    description?: string
    actionLabel?: string
    actionTo?: RouteLocationRaw
    actionDisabled?: boolean
    size?: Size
  }>(),
  {
    icon: 'inbox',
    size: 'md',
  },
)

defineEmits<{ action: [] }>()

const slots = useSlots()

const hasAction = computed(
  () => !!slots.action || !!props.actionTo || !!props.actionLabel,
)

const sizeClass = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'rounded-2xl p-8'
    case 'lg':
      return 'rounded-3xl p-16'
    default:
      return 'rounded-2xl p-12'
  }
})

const iconWrapperClass = computed(() => (props.size === 'sm' ? 'mb-3' : 'mb-5'))

const iconClass = computed(() => {
  const base = 'text-muted-foreground opacity-70'
  if (props.size === 'sm') return `w-12 h-12 ${base}`
  if (props.size === 'lg') return `w-24 h-24 ${base}`
  return `w-16 h-16 ${base}`
})

const titleClass = computed(() =>
  props.size === 'sm' ? 'text-sm' : props.size === 'lg' ? 'text-xl' : 'text-base',
)

const descriptionClass = computed(() =>
  props.size === 'sm' ? 'text-xs max-w-sm' : 'text-sm max-w-md',
)

const icons: Record<IconName, typeof Inbox> = {
  package: Package,
  cart: ShoppingCart,
  order: ClipboardList,
  search: Search,
  inbox: Inbox,
  alert: AlertCircle,
}

const resolvedIcon = computed(() => icons[props.icon] ?? Inbox)
</script>
