<script setup lang="ts">
import { RouterView } from 'vue-router'
import NoticeHost from '@/components/NoticeHost.vue'
import ConfirmDialogHost from '@/components/ConfirmDialogHost.vue'
import ReloginDialogHost from '@/components/ReloginDialogHost.vue'
</script>

<template>
  <RouterView />
  <NoticeHost />
  <ConfirmDialogHost />
  <ReloginDialogHost />
</template>
