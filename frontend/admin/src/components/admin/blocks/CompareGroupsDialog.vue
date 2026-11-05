<template>
  <Dialog v-model:open="open">
    <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-5xl p-4 sm:p-6">
      <DialogHeader>
        <DialogTitle>{{ t('admin.compareGroups.title') }}</DialogTitle>
        <DialogDescription>{{ t('admin.compareGroups.desc') }}</DialogDescription>
      </DialogHeader>
      <CompareGroupsPanel :lang="lang" :categories="categories" @changed="emit('changed')" />
    </DialogScrollContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Dialog, DialogDescription, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import CompareGroupsPanel from './CompareGroupsPanel.vue'

const props = defineProps<{ modelValue: boolean; lang: string; categories: Array<{ id: number; name: any }> }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; changed: [] }>()
const { t } = useI18n()

const open = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
})
</script>
