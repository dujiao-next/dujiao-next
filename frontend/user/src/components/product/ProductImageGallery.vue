<template>
  <div class="p-4 md:p-8 bg-secondary border-r">
    <div class="mb-4 md:mb-6 relative group"
      @touchstart="onImageTouchStart"
      @touchend="onImageTouchEnd">
      <img v-if="currentImage" :src="currentImage" :alt="productTitle"
        class="w-full aspect-[4/3] object-contain rounded-xl border relative z-10 shadow-lg" />
      <div v-else
        class="w-full aspect-[4/3] bg-muted rounded-xl border flex items-center justify-center relative z-10">
        <ImageIcon class="w-24 h-24 text-muted-foreground" :stroke-width="1.5" />
      </div>
    </div>

    <!-- Thumbnail Gallery: horizontal scroll on mobile, grid on desktop -->
    <div v-if="images.length > 1" class="flex md:grid md:grid-cols-5 gap-3 overflow-x-auto md:overflow-visible pb-2 md:pb-0 -mx-1 px-1 snap-x snap-mandatory">
      <div v-for="(image, index) in images" :key="index" @click="$emit('update:currentImage', image)"
        class="cursor-pointer rounded-lg overflow-hidden border-2 transition-all duration-300 shrink-0 w-16 h-16 md:w-auto md:h-auto md:aspect-square snap-start"
        :class="currentImage === image ? 'border-primary ring-1 ring-primary/40 opacity-100' : 'border-transparent opacity-60 hover:opacity-100 hover:scale-105'">
        <img :src="image" :alt="`Image ${index + 1}`" loading="lazy" class="w-full h-full object-cover" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Image as ImageIcon } from 'lucide-vue-next'

const props = defineProps<{
  images: string[]
  currentImage: string
  productTitle: string
}>()

const emit = defineEmits<{
  'update:currentImage': [value: string]
}>()

// Image touch swipe
let touchStartX = 0
const onImageTouchStart = (e: TouchEvent) => {
  touchStartX = e.touches[0]?.clientX ?? 0
}
const onImageTouchEnd = (e: TouchEvent) => {
  const touchEndX = e.changedTouches[0]?.clientX ?? 0
  const diff = touchStartX - touchEndX
  if (Math.abs(diff) < 50) return
  if (props.images.length <= 1) return
  const currentIdx = props.images.indexOf(props.currentImage)
  if (currentIdx === -1) return
  if (diff > 0) {
    // Swipe left -> next
    emit('update:currentImage', props.images[(currentIdx + 1) % props.images.length] ?? '')
  } else {
    // Swipe right -> prev
    emit('update:currentImage', props.images[(currentIdx - 1 + props.images.length) % props.images.length] ?? '')
  }
}
</script>
