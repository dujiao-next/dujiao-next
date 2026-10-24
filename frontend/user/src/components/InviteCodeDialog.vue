<template>
  <Teleport to="body">
    <div
      v-if="userAuthStore.inviteTicket"
      class="fixed inset-0 z-[110] flex items-center justify-center p-4"
      @keydown.esc="cancel"
    >
      <div class="fixed inset-0 bg-black/40 backdrop-blur-sm" aria-hidden="true" />
      <form
        ref="formRef"
        role="dialog"
        aria-modal="true"
        :aria-label="t('auth.invite.dialogTitle')"
        class="relative z-10 grid w-full max-w-sm gap-4 rounded-2xl border bg-card p-6 text-card-foreground shadow-2xl"
        @submit.prevent="submit"
      >
        <div>
          <h3 class="text-lg font-bold text-foreground">{{ t('auth.invite.dialogTitle') }}</h3>
          <p class="mt-2 text-sm leading-relaxed text-muted-foreground">{{ t('auth.invite.dialogMessage') }}</p>
        </div>
        <FormField :label="t('auth.invite.codeLabel')" required>
          <template #default="{ id }">
            <Input
              :id="id"
              v-model="code"
              autocomplete="off"
              class="h-11"
              :placeholder="t('auth.invite.codePlaceholder')"
            />
          </template>
        </FormField>
        <Alert v-if="error" variant="destructive" class="text-center">
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>
        <div class="flex items-center justify-end gap-3">
          <Button type="button" variant="secondary" @click="cancel">{{ t('common.cancel') }}</Button>
          <Button type="submit" :disabled="userAuthStore.loading || !code.trim()">
            {{ userAuthStore.loading ? t('auth.invite.submitting') : t('auth.invite.submit') }}
          </Button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUserAuthStore } from '../stores/userAuth'
import FormField from './FormField.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

/**
 * 邀请制注册：第三方登录返回 invite_required 后，凭 store 中的 inviteTicket 补填邀请码。
 * 有票据即显示；成功后把登录结果（可能需要 2FA）交给调用方继续。
 */
const emit = defineEmits<{
  success: [result: { requiresTotp: boolean }]
  cancel: []
}>()

const { t } = useI18n()
const userAuthStore = useUserAuthStore()
const code = ref('')
const error = ref('')
const formRef = ref<HTMLFormElement | null>(null)

watch(() => userAuthStore.inviteTicket, async (ticket) => {
  if (!ticket) return
  await nextTick()
  formRef.value?.querySelector('input')?.focus()
})

// 离开页面即作废未完成的票据，避免下次进入登录页时弹出过期对话框
onUnmounted(() => userAuthStore.clearInviteTicket())

const submit = async () => {
  if (userAuthStore.loading || !code.value.trim()) return
  error.value = ''
  try {
    const result = await userAuthStore.completeInviteRegistration(code.value.trim())
    code.value = ''
    emit('success', result)
  } catch (err: any) {
    error.value = err?.message || t('auth.invite.failed')
  }
}

const cancel = () => {
  userAuthStore.clearInviteTicket()
  code.value = ''
  error.value = ''
  emit('cancel')
}
</script>
