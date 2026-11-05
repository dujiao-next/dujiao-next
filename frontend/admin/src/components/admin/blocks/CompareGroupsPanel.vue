<template>
      <div class="mf">
        <!-- 左：组列表 -->
        <aside class="mf-list">
          <button
            v-for="group in groups"
            :key="group.id"
            type="button"
            class="mf-item"
            :class="{ 'is-active': current?.id === group.id }"
            @click="selectGroup(group)"
          >
            <span class="mf-item-name">{{ txt(group.name) || t('admin.compareGroups.untitled') }}</span>
            <span class="mf-item-meta">{{ dimsOf(group).length }} {{ t('admin.compareGroups.dims') }}</span>
          </button>

          <button type="button" class="mf-add" @click="createGroup">+ {{ t('admin.compareGroups.newGroup') }}</button>
        </aside>

        <!-- 右：维度编辑 -->
        <section v-if="current" class="mf-editor">
          <div class="mf-row">
            <label class="mf-field">
              <span>{{ t('admin.compareGroups.name') }}（{{ langLabel }}）</span>
              <Input v-model="current.name[lang]" :placeholder="t('admin.compareGroups.namePlaceholder')" />
            </label>
            <label class="mf-field">
              <span>{{ t('admin.compareGroups.category') }}</span>
              <Select
                :model-value="current.category_id != null ? String(current.category_id) : '__all__'"
                @update:model-value="(v) => (current!.category_id = v === '__all__' ? null : Number(v))"
              >
                <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all__">{{ t('admin.compareGroups.anyCategory') }}</SelectItem>
                  <SelectItem v-for="cat in categories" :key="cat.id" :value="String(cat.id)">
                    {{ getLocalizedText(cat.name) }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="mf-field mf-field--row">
              <Switch v-model="current.is_active" />
              <span>{{ t('admin.compareGroups.active') }}</span>
            </label>
          </div>

          <div class="mf-dims">
            <div class="mf-dims-head">
              <strong>{{ t('admin.compareGroups.dimensions') }}</strong>
              <Button size="sm" variant="outline" type="button" @click="addDimension">+ {{ t('admin.compareGroups.addDimension') }}</Button>
            </div>
            <p class="mf-dims-hint">{{ t('admin.symbolField.hint') }}</p>

            <div
              v-for="(dim, index) in dims"
              :key="dim.key"
              class="mf-dim"
              :class="{ 'is-dragging': dragIndex === index, 'is-drop-target': overIndex === index && dragIndex !== null }"
              @dragover.prevent="overIndex = index"
              @drop.prevent="dropAt(index)"
            >
              <span
                class="mf-grip"
                draggable="true"
                @dragstart="dragIndex = index"
                @dragend="resetDrag"
              >⠿</span>
              <span class="mf-dim-index">{{ index + 1 }}</span>

              <Input v-model="dim.label[lang]" class="mf-dim-label" :placeholder="t('admin.compareGroups.dimLabel')" />
              <SymbolField :model-value="dim.icon || ''" :show-hint="false" @update:model-value="(v) => (dim.icon = v)" />
              <Input v-model="dim.unit" class="mf-dim-unit" :placeholder="t('admin.compareGroups.dimUnit')" />

              <label class="mf-dim-visible" :title="t('admin.compareGroups.dimVisible')">
                <input type="checkbox" v-model="dim.visible" />
              </label>

              <button type="button" class="mf-dim-remove" @click="removeDimension(index)">✕</button>
            </div>

            <p v-if="!dims.length" class="mf-empty">{{ t('admin.compareGroups.empty') }}</p>
          </div>

          <div class="mf-foot">
            <span v-if="savedAt" class="mf-saved">{{ t('admin.compareGroups.saved') }} {{ savedAt }}</span>
            <Button v-if="current.id" size="sm" variant="destructive" type="button" @click="removeGroup">{{ t('admin.compareGroups.delete') }}</Button>
            <Button size="sm" type="button" :disabled="saving" @click="saveGroup">
              {{ saving ? t('admin.common.submitting') : t('admin.common.save') }}
            </Button>
          </div>
        </section>
      </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import SymbolField from '@/components/admin/blocks/SymbolField.vue'
import { getLocalizedText } from '@/utils/format'
import { notifyError, notifySuccess } from '@/utils/notify'

interface Dimension {
  key: string
  label: Record<string, string>
  icon: string
  unit: string
  type: string
  visible: boolean
  order: number
}

interface Group {
  id: number
  name: Record<string, string>
  category_id: number | null
  dimensions: { items?: Dimension[] }
  is_active: boolean
  sort_order: number
}

const props = defineProps<{ lang: string; categories: Array<{ id: number; name: any }> }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()

const groups = ref<Group[]>([])
const current = ref<Group | null>(null)
const saving = ref(false)
const savedAt = ref('')

const langLabel = computed(() => props.lang.toUpperCase())
const txt = (value: any) => {
  if (!value) return ''
  if (typeof value === 'string') return value
  return value[props.lang] || value['zh-CN'] || ''
}

const dimsOf = (group: Group): Dimension[] => group?.dimensions?.items || []
const dims = computed<Dimension[]>(() => (current.value ? dimsOf(current.value) : []))

const load = async () => {
  try {
    const res = await adminAPI.getCompareGroups({ silent: true })
    groups.value = (res.data.data || []) as Group[]
    if (current.value?.id) {
      current.value = groups.value.find((g) => g.id === current.value?.id) || groups.value[0] || null
    } else {
      current.value = groups.value[0] || null
    }
  } catch (err) {
    notifyError((err as Error).message)
  }
}

onMounted(load)

const selectGroup = (group: Group) => {
  current.value = JSON.parse(JSON.stringify(group))
  savedAt.value = ''
}

const createGroup = () => {
  current.value = {
    id: 0,
    name: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    category_id: null,
    dimensions: { items: [] },
    is_active: true,
    sort_order: groups.value.length,
  }
  savedAt.value = ''
}

const makeDimKey = () => `dim_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 7)}`

const addDimension = () => {
  if (!current.value) return
  const items = dimsOf(current.value)
  items.push({
    key: makeDimKey(),
    label: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    icon: '',
    unit: '',
    type: 'text',
    visible: true,
    order: items.length + 1,
  })
  current.value.dimensions = { items }
}

const removeDimension = (index: number) => {
  if (!current.value) return
  const items = dimsOf(current.value).filter((_, i) => i !== index)
  current.value.dimensions = { items }
}

/* 拖拽排序 */
const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)
const resetDrag = () => { dragIndex.value = null; overIndex.value = null }
const dropAt = (index: number) => {
  const from = dragIndex.value
  resetDrag()
  if (from === null || from === index || !current.value) return
  const items = [...dimsOf(current.value)]
  const [moved] = items.splice(from, 1)
  if (!moved) return
  items.splice(index, 0, moved)
  current.value.dimensions = { items }
}

const saveGroup = async () => {
  if (!current.value) return
  saving.value = true
  try {
    // 兜底：维度 key 为空或重复会导致商品取值互相覆盖
    const used = new Set<string>()
    const dimensions = dimsOf(current.value).map((dim, index) => {
      let key = String(dim.key || '').trim()
      if (!key || used.has(key)) key = makeDimKey()
      used.add(key)
      return { ...dim, key, order: index + 1 }
    })
    const payload = {
      name: current.value.name,
      category_id: current.value.category_id,
      dimensions: { items: dimensions },
      is_active: current.value.is_active,
      sort_order: current.value.sort_order,
    }
    if (current.value.id) {
      await adminAPI.updateCompareGroup(current.value.id, payload)
    } else {
      const res = await adminAPI.createCompareGroup(payload)
      current.value.id = (res.data?.data as any)?.id || 0
    }
    savedAt.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    notifySuccess(t('admin.compareGroups.saved'))
    emit('changed')
    await load()
  } catch (err) {
    notifyError((err as Error).message)
  } finally {
    saving.value = false
  }
}

const removeGroup = async () => {
  if (!current.value?.id) return
  if (!window.confirm(t('admin.compareGroups.confirmDelete'))) return
  try {
    await adminAPI.deleteCompareGroup(current.value.id)
    current.value = null
    emit('changed')
    await load()
    notifySuccess(t('admin.compareGroups.deleted'))
  } catch (err) {
    notifyError((err as Error).message)
  }
}
</script>

<style scoped>
.mf {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 16px;
  margin-top: 8px;
}
.mf-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-right: 1px solid var(--border, #e5e7eb);
  padding-right: 12px;
}
.mf-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: none;
  text-align: left;
  cursor: pointer;
}
.mf-item:hover { background: var(--muted, #f9fafb); }
.mf-item.is-active {
  border-color: var(--primary, #2563eb);
  background: rgba(37, 99, 235, 0.06);
}
.mf-item-name { font-size: 13px; font-weight: 600; }
.mf-item-meta { font-size: 11px; color: var(--muted-foreground, #9ca3af); }
.mf-add {
  margin-top: 6px;
  height: 32px;
  border: 1px dashed var(--border, #d1d5db);
  border-radius: 8px;
  background: none;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
  cursor: pointer;
}
.mf-editor { min-width: 0; }
.mf-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 200px auto;
  gap: 10px;
  align-items: end;
  margin-bottom: 14px;
}
.mf-field { display: block; }
.mf-field > span {
  display: block;
  margin-bottom: 5px;
  font-size: 12px;
  color: var(--muted-foreground, #6b7280);
}
.mf-field--row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 6px;
}
.mf-dims-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.mf-dims-hint {
  margin: -4px 0 8px;
  font-size: 11px;
  color: var(--muted-foreground, #9ca3af);
}
.mf-dim {
  display: grid;
  grid-template-columns: 16px 24px minmax(0, 1fr) 220px 86px 28px 24px;
  gap: 8px;
  align-items: center;
  padding: 6px 4px;
  border-bottom: 1px solid var(--border, #f1f5f9);
}
.mf-dim.is-dragging { opacity: 0.45; }
.mf-dim.is-drop-target { box-shadow: inset 0 2px 0 0 var(--primary, #2563eb); }
.mf-grip { color: var(--muted-foreground, #cbd5e1); cursor: grab; }
.mf-dim-index { font-size: 11px; color: var(--muted-foreground, #9ca3af); }
.mf-dim-icon { font-family: ui-monospace, monospace; font-size: 12px; }
.mf-dim-remove {
  border: 0;
  background: none;
  color: #dc2626;
  cursor: pointer;
}
.mf-empty {
  padding: 16px 0;
  font-size: 12px;
  color: var(--muted-foreground, #9ca3af);
}
.mf-foot {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 14px;
}
.mf-saved { margin-right: auto; font-size: 11px; color: var(--muted-foreground, #9ca3af); }
</style>
