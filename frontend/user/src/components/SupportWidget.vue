<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import supportMessages from '../i18n/locales/support'
import { MessageCircle, X, Send, ImagePlus, ArrowLeft, Plus } from 'lucide-vue-next'
import { api, userApi } from '../api/client'
import { useUserAuthStore } from '../stores/userAuth'
import { loadGuestOrderAuth } from '../utils/guestOrderAuth'

const { t } = useI18n({useScope:'local', messages:supportMessages})
type Conversation = {id:number;subject:string;status:string;unread_count:number;order_summary?:{order_no:string;status:string}}
type Message = {id:number;content:string;sender_type:string;created_at:string;attachments:{id:number}[]}
const auth = useUserAuthStore()
const open = ref(false), busy = ref(false), polling = ref(false), error = ref(''), draft = ref(''), category = ref('general'), orderNo = ref('')
const conversations = ref<Conversation[]>([]), selected = ref<Conversation|null>(null), messages = ref<Message[]>([]), pending = ref<number[]>([])
const imageURLs = ref<Record<number,string>>({}), scroller = ref<HTMLElement>()
let token = ''; try { token = sessionStorage.getItem('support_guest_token') || '' } catch {}
let generation = 0, timer: ReturnType<typeof setInterval>|undefined, pendingClientID = ''
const root = computed(() => auth.isAuthenticated ? '/support' : '/guest/support')
const client = computed(() => auth.isAuthenticated ? userApi : api)
const unread = computed(() => conversations.value.reduce((n,c)=>n+c.unread_count,0))
const opts = (): { headers: Record<string,string> } => ({headers: token && !auth.isAuthenticated ? {'X-Support-Token':token} : {}})
function clearImages() { Object.values(imageURLs.value).forEach(URL.revokeObjectURL); imageURLs.value={} }
function reset() {generation++;selected.value=null;messages.value=[];pending.value=[];conversations.value=[];error.value='';pendingClientID='';draft.value='';clearImages()}
watch(()=>auth.token, reset)
function failure(e:unknown) {error.value=e instanceof Error?e.message:t('error')}
async function refresh() {
  if (!open.value || polling.value || document.hidden) return
  polling.value=true;const g=generation
  try {
    const res=await client.value.get(root.value+'/conversations',opts());if(g!==generation)return
    conversations.value=res.data.data || []
    if (selected.value) {
      const id=selected.value.id; const detail=await client.value.get(`${root.value}/conversations/${id}`,opts());if(g!==generation||selected.value?.id!==id)return;selected.value=detail.data.data
      await loadMessages(id,g)
    }
  } catch(e) { if(g===generation) failure(e) } finally {polling.value=false}
}
async function loadMessages(id:number,g=generation) {
  const after=messages.value[messages.value.length-1]?.id||0
  const res=await client.value.get(`${root.value}/conversations/${id}/messages`,{...opts(),params:{after_id:after,limit:100}})
  if(g!==generation||selected.value?.id!==id)return
  const incoming:Message[]=res.data.data||[];const seen=new Set(messages.value.map(m=>m.id));messages.value.push(...incoming.filter(m=>!seen.has(m.id)))
  for(const m of incoming) for(const attachment of m.attachments||[]) {
    if(imageURLs.value[attachment.id])continue
    try {const blob=await client.value.get(`${root.value}/conversations/${id}/attachments/${attachment.id}`,{...opts(),blob:true});if(g===generation&&selected.value?.id===id) imageURLs.value[attachment.id]=URL.createObjectURL(blob.data)} catch{}
  }
  if(incoming.length && messages.value.length) await client.value.post(`${root.value}/conversations/${id}/read`,{message_id:messages.value[messages.value.length-1]!.id},opts())
  if(incoming.length){await nextTick();scroller.value?.scrollTo({top:scroller.value.scrollHeight})}
}
async function choose(c:Conversation) {generation++;clearImages();selected.value=c;messages.value=[];pending.value=[];error.value='';await refresh()}
function createNew() {generation++;selected.value=null;messages.value=[];pending.value=[];error.value='';clearImages()}
async function send() {
  if(busy.value||(!draft.value.trim()&&!pending.value.length))return
  busy.value=true;error.value='';const g=generation;const body=draft.value
  pendingClientID ||= crypto.randomUUID()
  try {
    if(!selected.value) {
      const res=await client.value.post(root.value+'/conversations',{subject:t(category.value),category:category.value,content:body,client_id:pendingClientID},opts())
      if(g!==generation)return
      selected.value=res.data.data.conversation
      if(res.data.data.token&&!auth.isAuthenticated){token=res.data.data.token;try{sessionStorage.setItem('support_guest_token',token)}catch{}}
    } else await client.value.post(`${root.value}/conversations/${selected.value.id}/messages`,{content:body,client_id:pendingClientID,attachment_ids:pending.value},opts())
    if(g!==generation)return
    draft.value='';pending.value=[];pendingClientID='';await refresh()
  }catch(e){if(g===generation)failure(e)}finally{busy.value=false}
}
async function upload(event:Event) {
  const input=event.target as HTMLInputElement,file=input.files?.[0];input.value=''
  if(!file||!selected.value||busy.value)return
  if(!['image/jpeg','image/png','image/webp'].includes(file.type)||file.size>5*1024*1024){error.value=t('imageError');return}
  busy.value=true;const g=generation
  try{const form=new FormData();form.append('file',file);const res=await client.value.post(`${root.value}/conversations/${selected.value.id}/attachments`,form,opts());if(g===generation)pending.value.push(res.data.data.id)}catch(e){failure(e)}finally{busy.value=false}
}
async function linkOrder() {
  if(!selected.value||!orderNo.value.trim()||busy.value)return
  busy.value=true
  try {
    const headers:Record<string,string>={...opts().headers}
    if(!auth.isAuthenticated){const credentials=loadGuestOrderAuth();if(!credentials.email||!credentials.order_password)throw new Error(t('orderHint'));const bytes=new TextEncoder().encode(credentials.email+'\n'+credentials.order_password);headers.Authorization='Guest '+btoa(String.fromCharCode(...bytes)).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'')}
    await client.value.post(`${root.value}/conversations/${selected.value.id}/order`,{order_no:orderNo.value.trim()},{headers});orderNo.value='';await refresh()
  }catch(e){failure(e)}finally{busy.value=false}
}
watch(open,v=>{if(v)void refresh()})
function openSupport(){open.value=true}
onMounted(()=>{window.addEventListener('support:open',openSupport);timer=setInterval(()=>void refresh(),4000)})
onUnmounted(()=>{window.removeEventListener('support:open',openSupport);generation++;clearInterval(timer);clearImages()})
</script>

<template>
  <div class="fixed bottom-20 right-4 z-50 lg:bottom-6 lg:right-6">
    <button v-if="!open" class="flex items-center gap-2 rounded-full bg-primary px-5 py-3 text-primary-foreground shadow-lg transition hover:-translate-y-1" :aria-label="t('title')" @click="open=true"><MessageCircle :size="20"/><span>{{t('title')}}</span><span v-if="unread" class="rounded-full bg-red-500 px-2 text-white">{{unread}}</span></button>
    <section v-else class="fixed inset-2 bottom-16 flex flex-col overflow-hidden rounded-2xl border border-border bg-background shadow-2xl sm:inset-auto sm:bottom-20 sm:right-5 sm:h-[620px] sm:max-h-[85vh] sm:w-[390px] lg:bottom-6" role="dialog" :aria-label="t('title')">
      <header class="flex items-center justify-between bg-primary px-4 py-4 text-primary-foreground"><div class="flex items-center gap-3"><button v-if="selected" :aria-label="t('back')" @click="createNew"><ArrowLeft :size="18"/></button><div><h2 class="font-semibold">{{t('title')}}</h2><p class="text-xs opacity-80">{{t('hint')}}</p></div></div><button :aria-label="t('close')" @click="open=false"><X :size="20"/></button></header>
      <div v-if="error" class="flex items-center gap-2 bg-destructive/10 px-4 py-2 text-xs text-destructive" role="alert">{{error}}<button class="underline" @click="error='';refresh()">{{t('retry')}}</button></div>
      <template v-if="!selected">
        <div class="overflow-auto p-4"><button v-for="c in conversations" :key="c.id" class="mb-2 flex w-full items-center justify-between rounded-xl border border-border p-3 text-left hover:bg-muted" @click="choose(c)"><span class="truncate text-sm">{{c.subject}} <small class="text-muted-foreground">#{{c.id}}</small></span><span v-if="c.unread_count" class="rounded-full bg-primary px-2 text-xs text-primary-foreground">{{c.unread_count}}</span></button></div>
        <div class="flex flex-1 flex-col justify-center px-6"><MessageCircle class="mb-4 text-primary" :size="34"/><h3 class="text-xl font-semibold">{{t('empty')}}</h3><p class="mt-2 text-xs leading-5 text-muted-foreground">{{t('privacy')}}</p><label class="mt-4 text-xs">{{t('category')}}</label><select v-model="category" class="mt-2 rounded-lg border border-border bg-background p-2 text-sm"><option v-for="key in ['general','presale','payment','delivery','aftersale']" :key="key" :value="key">{{t(key)}}</option></select></div>
      </template>
      <template v-else>
        <div class="flex items-center justify-between border-b px-4 py-2 text-xs"><span>CS-{{selected.id}} · {{t('status'+selected.status.charAt(0).toUpperCase()+selected.status.slice(1))}}</span><button @click="createNew" :aria-label="t('new')"><Plus :size="16"/></button></div>
        <div v-if="selected.order_summary" class="bg-muted/50 px-4 py-2 text-xs">{{t('linked')}}: {{selected.order_summary.order_no}}</div>
        <div ref="scroller" class="flex-1 space-y-3 overflow-auto px-4 py-4">
          <div v-for="m in messages" :key="m.id" class="flex" :class="m.sender_type==='admin'?'justify-start':'justify-end'"><div class="max-w-[85%] rounded-2xl px-3 py-2" :class="m.sender_type==='admin'?'bg-muted':'bg-primary text-primary-foreground'"><p class="whitespace-pre-wrap break-words text-sm">{{m.content}}</p><img v-for="a in m.attachments||[]" :key="a.id" :src="imageURLs[a.id]" class="mt-2 max-h-52 rounded-lg" :alt="t('attach')"/><time class="mt-1 block text-[10px] opacity-60">{{new Date(m.created_at).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'})}}</time></div></div>
        </div>
        <details class="border-t px-4 py-2 text-xs"><summary class="cursor-pointer text-muted-foreground">{{t('order')}}</summary><div class="mt-2 flex gap-2"><input v-model="orderNo" class="min-w-0 flex-1 rounded border bg-background px-2 py-1" :placeholder="t('orderHint')"/><button :disabled="busy" class="text-primary" @click="linkOrder">{{t('order')}}</button></div></details>
      </template>
      <form class="border-t bg-muted/20 p-3" @submit.prevent="send"><p v-if="pending.length" class="mb-2 text-xs">{{t('attach')}}: {{pending.length}} / 4</p><textarea v-model="draft" maxlength="4000" rows="2" class="w-full resize-none rounded-xl border border-border bg-background p-3 text-sm outline-none focus:border-primary" :placeholder="t('message')" :disabled="busy"/><div class="mt-2 flex items-center justify-between"><label v-if="selected" class="cursor-pointer text-muted-foreground"><ImagePlus :size="20"/><input type="file" class="hidden" accept="image/jpeg,image/png,image/webp" :disabled="busy||pending.length>=4" @change="upload"/></label><span v-else class="text-xs text-muted-foreground">{{t('new')}}</span><button class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground disabled:opacity-40" :disabled="busy||(!draft.trim()&&!pending.length)"><Send :size="14"/>{{t('send')}}</button></div></form>
    </section>
  </div>
</template>