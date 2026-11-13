<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AdminAPIToken, type AdminAuthzAdmin } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { confirmAction } from '@/utils/confirm'
import { copyText } from '@/utils/clipboard'
import { formatDate } from '@/utils/format'
import { notifyError, notifySuccess } from '@/utils/notify'
import { useAdminAuthStore } from '@/stores/auth'

const props = defineProps<{ admin: AdminAuthzAdmin }>()
const { t } = useI18n()
const auth = useAdminAuthStore()
const canCreate = computed(() => auth.hasPermission(`POST:/admin/authz/admins/${props.admin.id}/api-tokens`))
const canRevoke = computed(() => auth.hasPermission(`DELETE:/admin/authz/admins/${props.admin.id}/api-tokens/:token_id`))
const canRotate = computed(() => auth.hasPermission(`POST:/admin/authz/admins/${props.admin.id}/api-tokens/:token_id/rotate`))
const tokens = ref<AdminAPIToken[]>([])
const loading = ref(false)
const busy = ref(false)
const loadFailed = ref(false)
const revokingId = ref<number | null>(null)
const rotatingId = ref<number | null>(null)
const name = ref('')
const ipWhitelist = ref('')
const secret = ref('')
const secretTokenId = ref<number | null>(null)
const secretHeading = ref<HTMLElement | null>(null)
const defaultExpiry = new Date(Date.now() + 90 * 86400000)
const expiresAt = ref(new Date(defaultExpiry.getTime() - defaultExpiry.getTimezoneOffset() * 60000).toISOString().slice(0, 16))
let disposed = false

onBeforeUnmount(() => {
  disposed = true
  secret.value = ''
})

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const response = await adminAPI.listAdminAPITokens(props.admin.id)
    if (!disposed) tokens.value = response.data.data
  } catch {
    if (!disposed) loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function create() {
  if (busy.value || props.admin.is_super || secret.value || !canCreate.value) return
  const expiry = new Date(expiresAt.value)
  if (!name.value.trim() || !Number.isFinite(expiry.getTime()) || expiry.getTime() <= Date.now() || expiry.getTime() > Date.now() + 365 * 86400000) {
    notifyError(t('adminApiTokens.invalid'))
    return
  }
  busy.value = true
  try {
    const response = await adminAPI.createAdminAPIToken(props.admin.id, {
      name: name.value.trim(),
      expires_at: expiry.toISOString(),
      ip_whitelist: ipWhitelist.value.split(/\r?\n/).map((entry) => entry.trim()).filter(Boolean),
    })
    if (disposed) return
    secret.value = response.data.data.secret
    secretTokenId.value = response.data.data.token.id
    name.value = ''
    ipWhitelist.value = ''
    await nextTick()
    secretHeading.value?.focus()
    await load()
  } catch (error: unknown) {
    if (!disposed) notifyError(error instanceof Error ? error.message : t('adminApiTokens.failed'))
  } finally {
    busy.value = false
  }
}

async function dismissSecret() {
  secret.value = ''
  secretTokenId.value = null
  await nextTick()
  document.getElementById('admin-api-token-name')?.focus()
}

async function copy() {
  try {
    await copyText(secret.value)
    notifySuccess(t('adminApiTokens.copied'))
  } catch {
    notifyError(t('adminApiTokens.copyFailed'))
  }
}

async function revoke(token: AdminAPIToken) {
  if (busy.value || !canRevoke.value || !await confirmAction({
    title: t('adminApiTokens.revokeTitle'),
    description: t('adminApiTokens.revokeConfirm', { name: token.name }),
    confirmText: t('adminApiTokens.revokeTitle'),
  }) || disposed) return
  busy.value = true
  revokingId.value = token.id
  try {
    await adminAPI.revokeAdminAPIToken(props.admin.id, token.id)
    if (disposed) return
    if (secretTokenId.value === token.id) await dismissSecret()
    notifySuccess(t('adminApiTokens.revoked'))
    await load()
  } catch (error: unknown) {
    if (!disposed) notifyError(error instanceof Error ? error.message : t('adminApiTokens.failed'))
  } finally {
    busy.value = false
    revokingId.value = null
  }
}

async function rotate(token: AdminAPIToken) {
  if (busy.value || secret.value || props.admin.is_super || token.status !== 'active' || !canRotate.value || !await confirmAction({
    title: t('adminApiTokens.rotateTitle'),
    description: t('adminApiTokens.rotateConfirm', { name: token.name }),
    confirmText: t('adminApiTokens.rotate'),
  }) || disposed || busy.value || secret.value) return
  busy.value = true
  rotatingId.value = token.id
  try {
    const response = await adminAPI.rotateAdminAPIToken(props.admin.id, token.id)
    if (disposed) return
    secret.value = response.data.data.secret
    secretTokenId.value = response.data.data.token.id
    await nextTick()
    secretHeading.value?.focus()
    notifySuccess(t('adminApiTokens.rotated'))
    await load()
  } catch (error: unknown) {
    if (!disposed) notifyError(error instanceof Error ? error.message : t('adminApiTokens.failed'))
  } finally {
    busy.value = false
    rotatingId.value = null
  }
}

onMounted(load)
</script>

<template>
  <section class="min-w-0 space-y-6 rounded-xl border border-border bg-card p-4 sm:p-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0 space-y-2">
        <h2 class="break-words text-lg font-semibold">{{ t('adminApiTokens.title', { name: admin.username }) }}</h2>
        <p class="max-w-3xl text-sm leading-relaxed text-muted-foreground">{{ t('adminApiTokens.description') }}</p>
      </div>
      <Button variant="outline" :disabled="loading || busy" @click="load">{{ t('adminApiTokens.refresh') }}</Button>
    </div>
    <p v-if="admin.is_super" class="text-sm text-muted-foreground">{{ t('adminApiTokens.superHint') }}</p>

    <div v-if="secret" class="space-y-3 rounded-lg bg-muted/40 p-4">
      <h3 ref="secretHeading" tabindex="-1" class="font-semibold focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring">{{ t('adminApiTokens.once') }}</h3>
      <p class="text-sm text-muted-foreground">{{ t('adminApiTokens.onceHint') }}</p>
      <code class="block select-all break-all rounded-md bg-background p-3 text-sm">{{ secret }}</code>
      <div class="flex flex-wrap gap-2">
        <Button variant="outline" @click="copy">{{ t('adminApiTokens.copy') }}</Button>
        <Button @click="dismissSecret">{{ t('adminApiTokens.saved') }}</Button>
      </div>
    </div>
    <form v-else-if="!admin.is_super && canCreate" class="space-y-4" :aria-busy="busy" @submit.prevent="create">
      <div class="grid gap-4 sm:grid-cols-2">
        <div class="space-y-2">
          <Label for="admin-api-token-name">{{ t('adminApiTokens.name') }}</Label>
          <Input id="admin-api-token-name" v-model="name" class="text-base sm:text-sm" required maxlength="80" autocomplete="off" :disabled="busy" :placeholder="t('adminApiTokens.namePlaceholder')" />
        </div>
        <div class="space-y-2">
          <Label for="admin-api-token-expiry">{{ t('adminApiTokens.expiry') }}</Label>
          <Input id="admin-api-token-expiry" v-model="expiresAt" class="text-base sm:text-sm" type="datetime-local" required :disabled="busy" aria-describedby="admin-api-token-expiry-hint" />
          <p id="admin-api-token-expiry-hint" class="text-sm text-muted-foreground">{{ t('adminApiTokens.expiryHint') }}</p>
        </div>
      </div>
      <div class="space-y-2">
        <Label for="admin-api-token-ip-whitelist">{{ t('adminApiTokens.ipWhitelist') }}</Label>
        <Textarea id="admin-api-token-ip-whitelist" v-model="ipWhitelist" class="text-base sm:text-sm" :disabled="busy" :rows="3" :placeholder="t('adminApiTokens.ipWhitelistPlaceholder')" aria-describedby="admin-api-token-ip-hint" />
        <p id="admin-api-token-ip-hint" class="text-sm text-muted-foreground">{{ t('adminApiTokens.ipWhitelistHint') }}</p>
      </div>
      <Button type="submit" :disabled="busy">{{ t(busy && revokingId === null && rotatingId === null ? 'adminApiTokens.creating' : 'adminApiTokens.create') }}</Button>
    </form>

    <div class="border-t border-border pt-6" :aria-busy="loading">
      <div class="mb-4 flex flex-wrap items-baseline justify-between gap-2">
        <h3 class="font-semibold">{{ t('adminApiTokens.listTitle') }}</h3>
        <p class="text-xs text-muted-foreground">{{ t('adminApiTokens.lastUsedHint') }}</p>
      </div>
      <div v-if="loadFailed" role="alert" class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-md border border-border p-3">
        <p class="text-sm">{{ t('adminApiTokens.loadFailed') }}</p>
        <Button variant="outline" :disabled="busy" @click="load">{{ t('adminApiTokens.retry') }}</Button>
      </div>
      <p v-if="loading" role="status" class="py-4 text-sm text-muted-foreground">{{ t('adminApiTokens.loading') }}</p>
      <div v-else-if="!loadFailed && !tokens.length" class="space-y-2 py-4 text-sm text-muted-foreground">
        <p>{{ t('adminApiTokens.empty') }}</p>
        <p v-if="!admin.is_super && canCreate">{{ t('adminApiTokens.emptyHint') }}</p>
      </div>
      <ul class="divide-y divide-border">
        <li v-for="token in tokens" :key="token.id" class="space-y-4 py-5 first:pt-0">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 basis-full space-y-1 sm:flex-1 sm:basis-0">
              <h4 class="break-words font-medium">{{ token.name }}</h4>
              <p class="text-sm text-muted-foreground">#{{ token.id }} · {{ t(`adminApiTokens.states.${token.status}`) }}</p>
            </div>
            <div class="flex flex-wrap gap-2">
              <Button v-if="!admin.is_super && token.status === 'active' && canRotate" variant="outline" :disabled="busy || !!secret" :aria-label="t('adminApiTokens.rotateNamed', { name: token.name })" @click="rotate(token)">{{ t(rotatingId === token.id ? 'adminApiTokens.rotating' : 'adminApiTokens.rotate') }}</Button>
              <Button v-if="!token.revoked_at && canRevoke" variant="outline" :disabled="busy" :aria-label="t('adminApiTokens.revokeNamed', { name: token.name })" @click="revoke(token)">{{ t(revokingId === token.id ? 'adminApiTokens.revoking' : 'adminApiTokens.revoke') }}</Button>
            </div>
          </div>
          <dl class="grid gap-x-6 gap-y-4 text-sm sm:grid-cols-3">
            <div class="min-w-0 space-y-1">
              <dt class="text-muted-foreground">{{ t('adminApiTokens.createdAt') }}</dt>
              <dd class="tabular-nums">{{ formatDate(token.created_at) }}</dd>
            </div>
            <div class="min-w-0 space-y-1">
              <dt class="text-muted-foreground">{{ t('adminApiTokens.expiry') }}</dt>
              <dd class="tabular-nums">{{ formatDate(token.expires_at) }}</dd>
            </div>
            <div class="min-w-0 space-y-1">
              <dt class="text-muted-foreground">{{ t('adminApiTokens.lastUsed') }}</dt>
              <dd class="tabular-nums">{{ token.last_used_at ? formatDate(token.last_used_at) : t('adminApiTokens.never') }}</dd>
            </div>
            <div class="min-w-0 space-y-1 sm:col-span-2">
              <dt class="text-muted-foreground">{{ t('adminApiTokens.ipWhitelist') }}</dt>
              <dd>
                <details v-if="(token.ip_whitelist?.length || 0) > 2">
                  <summary class="cursor-pointer rounded-sm py-1 underline-offset-4 hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring">{{ t('adminApiTokens.showRules', { count: token.ip_whitelist!.length }) }}</summary>
                  <ul class="mt-2 space-y-1 break-all font-mono text-sm">
                    <li v-for="entry in token.ip_whitelist" :key="entry">{{ entry }}</li>
                  </ul>
                </details>
                <ul v-else-if="token.ip_whitelist?.length" class="space-y-1 break-all font-mono text-sm">
                  <li v-for="entry in token.ip_whitelist" :key="entry">{{ entry }}</li>
                </ul>
                <span v-else>{{ t('adminApiTokens.anyIP') }}</span>
              </dd>
            </div>
          </dl>
        </li>
      </ul>
    </div>
  </section>
</template>
