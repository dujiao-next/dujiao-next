<template>
  <transition name="fade">
    <div v-if="loading"
      class="fixed inset-0 z-50 flex items-center justify-center bg-background transition-opacity duration-300"
      role="status" aria-live="polite" aria-label="Dovelora loading">
      <div class="flex flex-col items-center gap-4">
        <img src="/site-icon.png" alt="" class="h-12 w-12 object-contain" />
        <span class="text-xs font-semibold tracking-[0.24em] text-foreground/70">DOVELORA</span>
        <div class="h-1 w-20 overflow-hidden rounded-full bg-muted">
          <div class="loading-progress h-full w-1/2 rounded-full bg-primary"></div>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
defineProps<{
  loading: boolean
}>()
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.loading-progress {
  animation: loading-progress 1.15s ease-in-out infinite;
}

@keyframes loading-progress {
  0% {
    transform: translateX(-110%);
  }
  100% {
    transform: translateX(210%);
  }
}
</style>
