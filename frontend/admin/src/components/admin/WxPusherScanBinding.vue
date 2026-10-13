<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { QrCode, RefreshCw, X } from 'lucide-vue-next'

const props = defineProps<{ active: boolean }>()
const emit = defineEmits<{ bound: [uid: string] }>()
const { t } = useI18n({useScope:'local',messages:{
  'zh-CN':{scan:'扫码订阅并绑定',hint:'先保存应用 App Token，再生成二维码。请用微信或 WxPusher 客户端扫码订阅，仅本人使用。',waiting:'等待扫码；每 11 秒检查一次',scanned:'已获取 UID，请输入 WxPusher 收到的验证码确认',expires:'剩余 {seconds} 秒',expired:'二维码已过期，请重新生成',cancelled:'扫码已取消，请重新生成',generate:'生成参数二维码',regenerate:'重新生成',confirm:'确认绑定',code:'8 位验证码',cancel:'取消扫码',failed:'二维码操作失败，请重试',image:'WxPusher 订阅二维码',open:'在手机上打开订阅链接',retry:'继续检查',confirmed:'订阅绑定成功'},
  'zh-TW':{scan:'掃碼訂閱並綁定',hint:'先儲存應用 App Token，再產生二維碼。請用微信或 WxPusher 用戶端掃碼訂閱，僅本人使用。',waiting:'等待掃碼；每 11 秒檢查一次',scanned:'已取得 UID，請輸入 WxPusher 收到的驗證碼確認',expires:'剩餘 {seconds} 秒',expired:'二維碼已過期，請重新產生',cancelled:'掃碼已取消，請重新產生',generate:'產生參數二維碼',regenerate:'重新產生',confirm:'確認綁定',code:'8 位驗證碼',cancel:'取消掃碼',failed:'二維碼操作失敗，請重試',image:'WxPusher 訂閱二維碼',open:'在手機上開啟訂閱連結',retry:'繼續檢查',confirmed:'訂閱綁定成功'},
  'en-US':{scan:'Subscribe and bind via QR',hint:'Save the app token first. Scan with WeChat or the WxPusher app to subscribe. Keep this QR code private.',waiting:'Waiting for scan; checking every 11 seconds',scanned:'UID received. Enter the verification code delivered by WxPusher',expires:'{seconds} seconds remaining',expired:'QR code expired. Generate a new one.',cancelled:'Scan cancelled. Generate a new QR code.',generate:'Generate subscription QR',regenerate:'Generate again',confirm:'Confirm binding',code:'8-character verification code',cancel:'Cancel scan',failed:'QR request failed. Please retry.',image:'WxPusher subscription QR code',open:'Open subscription link on your phone',retry:'Resume checking',confirmed:'Subscription bound successfully'}
}})
type Scan = {id:string;url:string;expires_at:string;status:'waiting'|'scanned'|'expired'|'cancelled'|'confirmed';uid?:string}
const scan = ref<Scan|null>(null), busy = ref(false), error = ref(''), code = ref(''), now = ref(Date.now())
const seconds = computed(()=>scan.value ? Math.max(0,Math.ceil((Date.parse(scan.value.expires_at)-now.value)/1000)) : 0)
const safeURL = computed(()=>{try{const url=new URL(scan.value?.url||'');return url.protocol==='https:'&&url.hostname==='wxpusher.zjiecode.com'&&!url.username?url.href:''}catch{return ''}})
let pollTimer:ReturnType<typeof setTimeout>|undefined, clock:ReturnType<typeof setInterval>|undefined, generation=0
const prefix='/admin/support/notification/scans'
function stop(){clearTimeout(pollTimer);clearInterval(clock);pollTimer=undefined;clock=undefined}
function report(e:unknown){error.value=e instanceof Error?e.message:t('failed')}
function schedule(){clearTimeout(pollTimer);if(props.active&&scan.value?.status==='waiting'&&seconds.value>0)pollTimer=setTimeout(()=>void poll(),11000)}
function startClock(){clearInterval(clock);clock=setInterval(()=>{now.value=Date.now();if(scan.value&&seconds.value===0&&['waiting','scanned'].includes(scan.value.status)){scan.value.status='expired';stop()}},1000)}
async function cancel(){generation++;stop();const current=scan.value;scan.value=null;code.value='';if(current&&['waiting','scanned'].includes(current.status)){try{await api.delete(`${prefix}/${current.id}`)}catch{}}}
async function generate(){if(busy.value)return;busy.value=true;error.value='';await cancel();const g=generation;try{const res=await api.post(prefix);const v:Scan=res.data.data;if(g!==generation||!props.active){void api.delete(`${prefix}/${v.id}`).catch(()=>{});return}scan.value=v;now.value=Date.now();startClock();schedule()}catch(e){if(g===generation)report(e)}finally{busy.value=false}}
async function poll(){if(!props.active||!scan.value||scan.value.status!=='waiting')return;if(document.hidden){schedule();return};const id=scan.value.id,g=generation;try{const res=await api.get(`${prefix}/${id}`);if(g!==generation||scan.value?.id!==id)return;scan.value=res.data.data;error.value='';if(scan.value?.status==='scanned')clearTimeout(pollTimer);else schedule()}catch(e){if(g===generation){report(e);schedule()}}}
async function confirm(){if(!scan.value||busy.value||scan.value.status!=='scanned'||seconds.value===0)return;busy.value=true;error.value='';const current=scan.value,g=generation;try{await api.post(`${prefix}/${current.id}/confirm`,{code:code.value.trim()});if(g!==generation)return;scan.value={...current,status:'confirmed'};code.value='';stop();emit('bound',current.uid||'')}catch(e){report(e)}finally{busy.value=false}}
watch(()=>props.active,v=>{if(!v)void cancel()})
onUnmounted(()=>{void cancel()})
</script>

<template>
  <div class="mb-4 rounded-xl border bg-muted/20 p-4">
    <h3 class="flex items-center gap-2 text-sm font-semibold"><QrCode :size="18"/>{{t('scan')}}</h3>
    <p class="mt-2 text-xs leading-5 text-muted-foreground">{{t('hint')}}</p>
    <p v-if="error" role="alert" class="mt-2 text-sm text-destructive">{{error}}</p>
    <button v-if="!scan||['expired','cancelled'].includes(scan.status)" class="mt-3 flex items-center gap-2 rounded-lg bg-primary px-3 py-2 text-sm text-primary-foreground disabled:opacity-40" :disabled="busy" @click="generate"><RefreshCw :size="14"/>{{scan?t('regenerate'):t('generate')}}</button>
    <div v-if="scan" class="mt-3 space-y-3">
      <template v-if="scan.status==='waiting' && safeURL">
        <img :src="safeURL" :alt="t('image')" referrerpolicy="no-referrer" class="h-52 w-52 rounded-lg border bg-white p-2"/>
        <p class="text-xs text-muted-foreground">{{t('waiting')}}</p>
      </template>
      <p v-if="['waiting','scanned'].includes(scan.status)" class="text-xs text-muted-foreground">{{t('expires',{seconds})}}</p>
      <template v-if="scan.status==='scanned'"><p class="text-sm">{{t('scanned')}}</p><p class="break-all text-xs">{{scan.uid}}</p><div class="flex gap-2"><input v-model="code" autocomplete="one-time-code" maxlength="8" class="min-w-0 flex-1 rounded-lg border bg-background px-3 py-2 text-sm" :placeholder="t('code')"/><button class="rounded-lg bg-primary px-3 py-2 text-sm text-primary-foreground disabled:opacity-40" :disabled="busy||code.trim().length!==8" @click="confirm">{{t('confirm')}}</button></div></template>
      <p v-if="['expired','cancelled','confirmed'].includes(scan.status)" class="text-sm" role="status">{{t(scan.status)}}</p>
      <button v-if="['waiting','scanned'].includes(scan.status)" class="flex items-center gap-1 text-xs text-muted-foreground" @click="cancel"><X :size="14"/>{{t('cancel')}}</button>
    </div>
  </div>
</template>