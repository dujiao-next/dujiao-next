<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'

const emit=defineEmits<{bound:[]}>()
const {t}=useI18n({useScope:'local',messages:{
 'zh-CN':{title:'SPT 极简模式',hint:'无需创建应用或填写 App Token。扫码获取你的 SPT，再粘贴到下方。系统无法自动读取该二维码的扫码结果。',secret:'SPT 是推送密钥，请勿分享。仅加密保存，绑定后不回显。',input:'粘贴 SPT_ 开头的密钥',verify:'发送验证消息',code:'收到的 8 位验证码',confirm:'确认绑定',qr:'WxPusher 获取 SPT 二维码',open:'打开官方 SPT 获取页',failed:'绑定失败，请重试',pending:'验证消息已发送，请查看 WxPusher。全局“启用提醒”仍需由主管打开。'},
 'zh-TW':{title:'SPT 極簡模式',hint:'無需建立應用或填寫 App Token。掃碼取得你的 SPT，再貼至下方。系統無法自動讀取此二維碼的掃碼結果。',secret:'SPT 是推送金鑰，請勿分享。僅加密儲存，綁定後不回顯。',input:'貼上 SPT_ 開頭的金鑰',verify:'發送驗證訊息',code:'收到的 8 位驗證碼',confirm:'確認綁定',qr:'WxPusher 取得 SPT 二維碼',open:'開啟官方 SPT 取得頁',failed:'綁定失敗，請重試',pending:'驗證訊息已發送，請查看 WxPusher。全域「啟用提醒」仍需由主管開啟。'},
 'en-US':{title:'SPT simple mode',hint:'No app or App Token required. Scan to obtain your SPT and paste it below. This QR does not automatically return a scan result to this store.',secret:'SPT is a push secret. Keep it private. It is stored encrypted and never displayed after binding.',input:'Paste your SPT_ secret',verify:'Send verification',code:'8-character code received',confirm:'Confirm binding',qr:'WxPusher obtain SPT QR',open:'Open official SPT page',failed:'Binding failed. Please retry.',pending:'Verification sent. Check WxPusher. A supervisor must still enable notifications globally.'}
}})
const qr='https://wxpusher.zjiecode.com/api/qrcode/RwjGLMOPTYp35zSYQr0HxbCPrV9eU0wKVBXU1D5VVtya0cQXEJWPjqBdW3gKLifS.jpg'
const spt=ref(''),code=ref(''),busy=ref(false),pending=ref(false),error=ref('')
async function begin(){if(busy.value)return;busy.value=true;error.value='';try{await api.post('/admin/support/notification/binding',{mode:'simple',spt:spt.value.trim()});spt.value='';pending.value=true}catch(e){error.value=e instanceof Error?e.message:t('failed')}finally{busy.value=false}}
async function confirm(){if(busy.value)return;busy.value=true;error.value='';try{await api.post('/admin/support/notification/binding/confirm',{code:code.value.trim()});code.value='';pending.value=false;emit('bound')}catch(e){error.value=e instanceof Error?e.message:t('failed')}finally{busy.value=false}}
</script>
<template>
 <div class="mb-4 space-y-3 rounded-xl border bg-muted/20 p-4">
  <h3 class="text-sm font-semibold">{{t('title')}}</h3><p class="text-xs leading-5 text-muted-foreground">{{t('hint')}}</p>
  <img :src="qr" :alt="t('qr')" referrerpolicy="no-referrer" class="h-48 w-48 rounded-lg border bg-white p-2"/>
  <a :href="qr" target="_blank" rel="noopener noreferrer" class="block text-xs text-primary">{{t('open')}}</a>
  <p class="text-xs text-muted-foreground">{{t('secret')}}</p><p v-if="error" role="alert" class="text-sm text-destructive">{{error}}</p>
  <input v-model="spt" type="password" autocomplete="new-password" maxlength="512" class="w-full rounded-lg border bg-background px-3 py-2 text-sm" :placeholder="t('input')"/>
  <button :disabled="busy||!spt.trim().startsWith('SPT_')" class="rounded-lg bg-primary px-3 py-2 text-sm text-primary-foreground disabled:opacity-40" @click="begin">{{t('verify')}}</button>
  <div v-if="pending" class="space-y-2"><p class="text-xs text-muted-foreground">{{t('pending')}}</p><div class="flex gap-2"><input v-model="code" maxlength="8" autocomplete="one-time-code" class="min-w-0 flex-1 rounded-lg border bg-background px-3 py-2 text-sm" :placeholder="t('code')"/><button :disabled="busy||code.trim().length!==8" class="rounded-lg border px-3 py-2 text-sm" @click="confirm">{{t('confirm')}}</button></div></div>
 </div>
</template>