<template>
  <section class="rounded-2xl border bg-secondary p-5 sm:p-6" aria-live="polite" data-testid="fixed-usdt-cashier">
    <template v-if="details">
      <div class="text-sm text-muted-foreground">{{ t('payment.fixed.amountDue') }}</div>
      <div class="mt-2 break-all text-3xl font-bold tabular-nums">{{ details.amount }} USDT</div>
      <div class="mt-2 font-semibold">{{ networkLabel }}</div>
      <p class="mt-3 text-sm text-muted-foreground">{{ t('payment.fixed.exactAmountTip') }}</p>
      <div class="mt-5 flex flex-col items-center gap-3">
        <img v-if="qrImage" :src="qrImage" :alt="t('payment.fixed.qrAlt')" class="h-56 w-56 rounded-xl bg-white p-2" />
        <p class="text-xs text-muted-foreground">{{ t('payment.fixed.qrAddressOnly') }}</p>
      </div>
      <div class="mt-5 text-xs text-muted-foreground">{{ t('payment.walletAddress') }}</div>
      <div class="mt-1 break-all font-mono text-sm" data-testid="fixed-recipient">{{ details.recipient }}</div>
      <div class="mt-3 flex flex-wrap gap-2">
        <Button variant="outline" size="sm" @click="copy('recipient')">{{ t('payment.copyWalletAddress') }}</Button>
        <Button variant="outline" size="sm" @click="copy('amount')">{{ t('payment.fixed.copyAmount') }}</Button>
        <span v-if="copied" class="self-center text-xs text-success">{{ t('payment.copied') }}</span>
      </div>
      <div class="mt-4 break-all text-xs text-muted-foreground">{{ t('payment.fixed.contract') }}：{{ details.contract }}</div>
      <p class="mt-4 text-sm">{{ t('payment.fixed.waitForConfirmation') }}</p>
      <p class="mt-2 text-xs text-muted-foreground">{{ t('payment.fixed.latePayment') }}</p>
    </template>
    <template v-else>
      <p class="font-semibold" role="status">{{ t(`payment.fixed.states.${displayState}`) }}</p>
      <p class="mt-2 text-sm text-muted-foreground">{{ t('payment.fixed.doNotPay') }}</p>
    </template>
    <p v-if="copyError" class="mt-3 text-sm text-destructive">{{ t('payment.copyFailed') }}</p>
    <Button class="mt-5" variant="outline" size="sm" :disabled="busy" @click="refresh">{{ t('payment.refreshStatus') }}</Button>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import { Button } from '@/components/ui/button'
import { paymentAPI } from '../../api'
import { copyText } from '../../utils/clipboard'
import { parseCashierSnapshot, visibleCashierDetails, type CashierSnapshot } from '../../utils/fixedCashier'

const props = defineProps<{ paymentId: number; orderNo: string }>()
const emit = defineEmits<{ refreshOrder: [] }>()
const { t } = useI18n()
const snapshot = ref<CashierSnapshot>({ state: 'unavailable', details: null, deadline: 0 })
const now = ref(performance.now())
const busy = ref(false)
const copied = ref(false)
const copyError = ref(false)
const qrImage = ref('')
const details = computed(() => visibleCashierDetails(snapshot.value, now.value))
const displayState = computed(() => snapshot.value.state === 'ready' ? 'unavailable' : snapshot.value.state)
const networkLabel = computed(() => details.value?.network === 'tron' ? 'TRON · TRC20' : 'BNB Smart Chain · BEP20')
let generation = 0
let stopped = false
let timer: ReturnType<typeof setInterval> | undefined
let poll: ReturnType<typeof setInterval> | undefined

function clear() {
  generation++
  busy.value = false
  snapshot.value = { state: 'unavailable', details: null, deadline: 0 }
  qrImage.value = ''
  copied.value = false
}

async function refresh() {
  if (stopped || busy.value || document.hidden || !navigator.onLine || !props.paymentId || !props.orderNo) return
  const ticket = ++generation
  const id = props.paymentId
  const orderNo = props.orderNo
  const started = performance.now()
  busy.value = true
  try {
    const response = await paymentAPI.cashier({ order_no: orderNo, payment_id: id })
    if (ticket !== generation || stopped) return
    now.value = performance.now()
    snapshot.value = parseCashierSnapshot(response.data.data, id, started, now.value)
    if (snapshot.value.state === 'closed') emit('refreshOrder')
  } catch {
    if (ticket === generation) snapshot.value = { state: 'unavailable', details: null, deadline: 0 }
  } finally {
    if (ticket === generation) busy.value = false
  }
}

watch(() => [props.paymentId, props.orderNo], () => { clear(); void refresh() })
watch(details, async (value) => {
  copied.value = false
  qrImage.value = ''
  if (!value) return
  // QR contains only the validated address, never a remote image or wallet URI.
  const image = await QRCode.toDataURL(value.recipient, { width: 224, margin: 1, errorCorrectionLevel: 'M' }).catch(() => '')
  if (details.value === value && visibleCashierDetails(snapshot.value, performance.now())) qrImage.value = image
})

async function copy(field: 'recipient' | 'amount') {
  copied.value = false
  copyError.value = false
  const current = visibleCashierDetails(snapshot.value, performance.now())
  if (!current || document.hidden || !navigator.onLine) { clear(); return }
  try { await copyText(current[field]); copied.value = true } catch { copyError.value = true }
}

function visibility() { clear(); if (!document.hidden) void refresh() }
function offline() { clear() }
function online() { clear(); void refresh() }
onMounted(() => {
  void refresh()
  timer = setInterval(() => { now.value = performance.now() }, 250)
  poll = setInterval(() => { void refresh() }, 5000)
  document.addEventListener('visibilitychange', visibility)
  window.addEventListener('offline', offline)
  window.addEventListener('online', online)
  window.addEventListener('pagehide', offline)
  window.addEventListener('pageshow', online)
})
onUnmounted(() => {
  stopped = true
  clear()
  clearInterval(timer)
  clearInterval(poll)
  document.removeEventListener('visibilitychange', visibility)
  window.removeEventListener('offline', offline)
  window.removeEventListener('online', online)
  window.removeEventListener('pagehide', offline)
  window.removeEventListener('pageshow', online)
})
</script>
