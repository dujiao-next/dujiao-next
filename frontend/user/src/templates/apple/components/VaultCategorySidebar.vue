<template>
  <aside class="min-w-0 lg:sticky lg:top-[88px]">
    <div class="rounded-xl border bg-card p-2.5 sm:p-4">
      <div class="mb-3 hidden items-center gap-2 px-1 lg:flex">
        <span class="h-4 w-1 flex-none rounded-full bg-primary"></span>
        <h4 class="text-sm font-bold">{{ t('products.categories') }}</h4>
      </div>
      <!-- 移动端横向 chips / 桌面竖向列表 -->
      <div class="flex gap-1.5 overflow-x-auto pb-0.5 lg:grid lg:gap-1 lg:overflow-visible lg:pb-0">
        <button
          type="button"
          class="flex-none whitespace-nowrap rounded-full px-3.5 py-2 text-sm font-semibold transition-colors lg:w-full lg:rounded-lg lg:px-3 lg:py-2.5 lg:text-left"
          :class="selectedCategory === null
            ? 'bg-primary text-white lg:bg-primary/10 lg:text-primary'
            : 'bg-secondary text-muted-foreground hover:text-foreground lg:bg-transparent lg:hover:bg-secondary'"
          @click="$emit('select', null)"
        >
          {{ t('products.allCategories') }}
        </button>
        <template v-for="grp in categoryGroups" :key="grp.id">
          <div class="flex flex-none items-center gap-0.5 lg:w-full lg:min-w-0">
            <button
              type="button"
              class="flex min-w-0 max-w-[65vw] flex-none items-center gap-2.5 rounded-full px-3.5 py-2 text-sm font-semibold transition-colors lg:w-full lg:max-w-none lg:flex-1 lg:rounded-lg lg:px-3 lg:py-2.5 lg:text-left"
              :class="selectedCategory === grp.id
                ? 'bg-primary text-white lg:bg-primary/10 lg:text-primary'
                : 'bg-secondary text-muted-foreground hover:text-foreground lg:bg-transparent lg:hover:bg-secondary'"
              @click="$emit('select', grp.id)"
            >
              <img v-if="grp.icon" :src="getImageUrl(grp.icon)" :alt="catName(grp)" loading="lazy" class="h-5 w-5 flex-none rounded-md object-cover" />
              <span class="min-w-0 truncate">{{ catName(grp) }}</span>
            </button>
            <button
              v-if="grp.children.length"
              type="button"
              class="hidden h-9 w-9 flex-none place-items-center rounded-lg transition-colors hover:bg-secondary hover:text-foreground lg:grid"
              :class="expandedParentIds.includes(grp.id) ? 'text-primary' : 'text-muted-foreground'"
              :aria-expanded="expandedParentIds.includes(grp.id)"
              :aria-label="catName(grp)"
              @click="$emit('toggle', grp.id)"
            >
              <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': expandedParentIds.includes(grp.id) }" />
            </button>
          </div>
          <template v-if="grp.children.length && expandedParentIds.includes(grp.id)">
            <button
              v-for="child in grp.children"
              :key="child.id"
              type="button"
              class="hidden w-full min-w-0 truncate rounded-lg py-2 pl-9 pr-3 text-left text-[13.5px] font-semibold transition-colors lg:block"
              :class="selectedCategory === child.id ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
              @click="$emit('select', child.id)"
            >
              {{ catName(child) }}
            </button>
          </template>
        </template>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronDown } from 'lucide-vue-next'
import { getImageUrl } from '../../../utils/image'
import { useLocalized } from '../../../composables/useProduct'
import type { PublicCategory } from '../../../utils/category'

defineProps<{
  categoryGroups: (PublicCategory & { children: PublicCategory[] })[]
  selectedCategory: number | null
  expandedParentIds: number[]
}>()

defineEmits<{ select: [id: number | null]; toggle: [id: number] }>()

const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const catName = (cat: PublicCategory) => getLocalizedText(cat.name) || cat.slug || ''
</script>
