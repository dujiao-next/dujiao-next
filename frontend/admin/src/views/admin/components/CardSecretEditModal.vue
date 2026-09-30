<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminCardSecret } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const props = defineProps<{
  modelValue: boolean
  cardSecret: AdminCardSecret | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  success: []
}>()

const { t } = useI18n()

const editSubmitting = ref(false)
const editError = ref('')
const editForm = reactive({
  id: 0,
  secret: '',
  status: 'available',
  is_loop: false,
})

watch(
  () => props.cardSecret,
  (secret) => {
    if (secret) {
      editForm.id = secret.id
      editForm.secret = secret.secret || ''
      editForm.status = secret.status || 'available'
      editForm.is_loop = Boolean(secret.is_loop)
      editError.value = ''
    }
  },
)

const closeModal = () => {
  emit('update:modelValue', false)
  editError.value = ''
}

const handleOpenChange = (value: boolean) => {
  if (!value) closeModal()
}

const submitEdit = async () => {
  if (!editForm.id) return
  editSubmitting.value = true
  editError.value = ''
  try {
    await adminAPI.updateCardSecret(editForm.id, {
      secret: editForm.secret,
      status: editForm.status,
      is_loop: editForm.is_loop,
    })
    closeModal()
    emit('success')
  } catch (error: any) {
    editError.value = error?.message || t('admin.cardSecrets.errors.updateFailed')
  } finally {
    editSubmitting.value = false
  }
}
</script>

<template>
  <Dialog :open="modelValue" @update:open="handleOpenChange">
    <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-lg p-4 sm:p-6">
      <DialogHeader>
        <DialogTitle>{{ t('admin.cardSecrets.editTitle') }}</DialogTitle>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="submitEdit">
        <div class="text-xs text-muted-foreground flex items-center gap-2">
          <span>{{ t('admin.cardSecrets.editId') }}:</span>
          <IdCell v-if="editForm.id" :value="editForm.id" />
          <span v-else>-</span>
        </div>
        <div>
          <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.cardSecrets.editSecret') }}</label>
          <Textarea v-model="editForm.secret" rows="3" :placeholder="t('admin.cardSecrets.editSecretPlaceholder')" />
        </div>
        <div>
          <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.cardSecrets.editStatus') }}</label>
          <Select v-model="editForm.status">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.cardSecrets.status.available')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="available">{{ t('admin.cardSecrets.status.available') }}</SelectItem>
              <SelectItem value="reserved">{{ t('admin.cardSecrets.status.reserved') }}</SelectItem>
              <SelectItem value="used">{{ t('admin.cardSecrets.status.used') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="flex items-start justify-between gap-4 border-y border-border py-3">
          <div>
            <label for="card-secret-edit-is-loop" class="text-sm font-medium text-foreground">
              {{ t('admin.cardSecrets.isLoopLabel') }}
            </label>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.cardSecrets.isLoopHint') }}</p>
          </div>
          <!-- 已占用的卡密属于待支付订单，后端不允许切换循环状态 -->
          <Switch
            id="card-secret-edit-is-loop"
            v-model="editForm.is_loop"
            class="mt-0.5"
            :disabled="editForm.status === 'reserved'"
          />
        </div>
        <div v-if="editError" class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
          {{ editError }}
        </div>
        <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <Button class="w-full sm:w-auto" type="button" variant="outline" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
          <Button class="w-full sm:w-auto" type="submit" :disabled="editSubmitting">
            {{ editSubmitting ? t('admin.common.loading') : t('admin.common.save') }}
          </Button>
        </div>
      </form>
    </DialogScrollContent>
  </Dialog>
</template>
