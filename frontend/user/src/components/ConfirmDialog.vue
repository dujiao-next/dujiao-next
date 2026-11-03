<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="visible"
        class="fixed inset-0 z-[110] flex items-center justify-center p-4"
        @click.self="handleCancel"
      >
        <div class="fixed inset-0 bg-black/40 backdrop-blur-[var(--modal-scrim-blur,4px)]" aria-hidden="true" />
        <Transition
          enter-active-class="transition duration-200 ease-out"
          enter-from-class="opacity-0 scale-95 translate-y-2"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-150 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 translate-y-2"
        >
          <div
            v-if="visible"
            ref="dialogRef"
            role="alertdialog"
            aria-modal="true"
            class="relative z-10 w-full max-w-sm rounded-2xl bg-card text-card-foreground border shadow-2xl p-6"
          >
            <h3 class="text-lg font-bold text-foreground">{{ options.title }}</h3>
            <p class="mt-2 text-sm text-muted-foreground leading-relaxed">{{ options.message }}</p>
            <div class="mt-6 flex items-center justify-end gap-3">
              <Button variant="secondary" @click="handleCancel">
                {{ options.cancelText || t('common.cancel') }}
              </Button>
              <Button
                :variant="options.variant === 'danger' ? 'destructive' : 'default'"
                @click="handleConfirm"
              >
                {{ options.confirmText || t('common.confirm') }}
              </Button>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import { Button } from '@/components/ui/button'

const { t } = useI18n()
const { visible, options, handleConfirm, handleCancel } = useConfirmDialog()

const dialogRef = ref<HTMLElement | null>(null)

const onKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') {
    handleCancel()
  }
}

watch(visible, async (isVisible) => {
  if (isVisible) {
    document.addEventListener('keydown', onKeydown)
    await nextTick()
    const firstButton = dialogRef.value?.querySelector('button') as HTMLElement | null
    firstButton?.focus()
  } else {
    document.removeEventListener('keydown', onKeydown)
  }
})
</script>
