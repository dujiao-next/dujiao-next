<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle, CheckCircle2, Info, Megaphone, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useLocalized } from '../composables/useProduct'
import { sanitizeRichHtml } from '../utils/richContent'
import { useAnnouncement, type HomeAnnouncement } from '../composables/useAnnouncement'

const props = defineProps<{
  announcement: HomeAnnouncement
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const { dismissForever, dismissToday, closeForSession } = useAnnouncement()

const title = computed(() => getLocalizedText(props.announcement.title))

const sanitizedContent = computed(() => {
  const raw = getLocalizedText(props.announcement.content)
  return sanitizeRichHtml(raw)
})

const typeStyle = computed(() => {
  switch (props.announcement.type) {
    case 'warning':
      return {
        iconWrap: 'bg-warning-soft text-warning ring-warning/20',
        icon: AlertTriangle,
      }
    case 'info':
      return {
        iconWrap: 'bg-info-soft text-info ring-info/20',
        icon: Info,
      }
    case 'success':
      return {
        iconWrap: 'bg-success-soft text-success ring-success/20',
        icon: CheckCircle2,
      }
    default:
      return {
        iconWrap: 'bg-primary-soft text-primary ring-primary/20',
        icon: Megaphone,
      }
  }
})

const close = () => emit('update:visible', false)

const handleClose = () => {
  closeForSession(props.announcement.version)
  close()
}

const handleDismissToday = () => {
  dismissToday(props.announcement.version)
  close()
}

const handleDismissForever = () => {
  dismissForever(props.announcement.version)
  close()
}

const dialogRef = ref<HTMLElement | null>(null)

const onKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') {
    handleClose()
  }
}

watch(
  () => props.visible,
  async (isVisible) => {
    if (isVisible) {
      document.addEventListener('keydown', onKeydown)
      await nextTick()
      const firstButton = dialogRef.value?.querySelector('button') as HTMLElement | null
      firstButton?.focus()
    } else {
      document.removeEventListener('keydown', onKeydown)
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-opacity duration-300 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-200 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="visible"
        class="fixed inset-0 z-[120] flex items-center justify-center p-4 sm:p-6"
        @click.self="handleClose"
      >
        <div class="fixed inset-0 bg-black/55 backdrop-blur-md" aria-hidden="true" />
        <Transition
          enter-active-class="transition duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)]"
          enter-from-class="opacity-0 scale-95 translate-y-3"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-200 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 translate-y-2"
        >
          <div
            v-if="visible"
            ref="dialogRef"
            role="dialog"
            aria-modal="true"
            aria-labelledby="announcement-modal-title"
            class="bg-card text-card-foreground relative z-10 flex max-h-[86vh] w-full max-w-lg flex-col overflow-hidden rounded-3xl border"
            style="box-shadow: var(--ui-shadow-card)"
          >
            <!-- Header -->
            <div class="flex items-center gap-4 px-6 pb-5 pt-6">
              <div
                class="flex size-12 shrink-0 items-center justify-center rounded-2xl ring-1 ring-inset"
                :class="typeStyle.iconWrap"
              >
                <component :is="typeStyle.icon" class="size-6" :stroke-width="2" />
              </div>
              <h3
                id="announcement-modal-title"
                class="line-clamp-2 min-w-0 flex-1 text-lg font-semibold leading-snug text-foreground"
              >
                {{ title }}
              </h3>
              <button
                type="button"
                class="flex size-8 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                :aria-label="t('announcement.close')"
                @click="handleClose"
              >
                <X class="size-4" :stroke-width="2" />
              </button>
            </div>

            <!-- Body -->
            <div class="min-h-0 flex-1 overflow-y-auto px-6 pb-2">
              <div class="theme-prose prose prose-sm max-w-none dark:prose-invert" v-html="sanitizedContent"></div>
            </div>

            <!-- Footer -->
            <div class="flex flex-col gap-3 border-t px-6 pb-5 pt-4 sm:flex-row sm:items-center sm:justify-between">
              <div class="flex items-center justify-center gap-2 text-xs sm:justify-start">
                <Button
                  variant="link"
                  class="h-auto p-0 text-xs font-normal text-muted-foreground hover:text-foreground"
                  @click="handleDismissToday"
                >
                  {{ t('announcement.dismissToday') }}
                </Button>
                <span class="select-none opacity-30 text-muted-foreground">·</span>
                <Button
                  variant="link"
                  class="h-auto p-0 text-xs font-normal text-muted-foreground hover:text-foreground"
                  @click="handleDismissForever"
                >
                  {{ t('announcement.dismissForever') }}
                </Button>
              </div>
              <Button class="w-full rounded-xl sm:w-auto" @click="handleClose">
                {{ t('announcement.close') }}
              </Button>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
