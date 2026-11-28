import { onBeforeUnmount, watch } from 'vue'

const STORAGE_PREFIX = 'admin-form-draft:'

const storage = () => (typeof sessionStorage === 'undefined' ? null : sessionStorage)

const keyOf = (key: string) => `${STORAGE_PREFIX}${key}`

export interface FormDraftOptions<T> {
  /** 当前草稿键；返回空串表示此刻不需要保存（例如弹窗未打开）。 */
  key: () => string
  /** 采集当前表单内容。 */
  snapshot: () => T
  /** 把草稿写回表单。 */
  apply: (draft: T) => void
  /** 判断快照是否有实际内容，避免把空表单存成草稿。 */
  hasContent: (draft: T) => boolean
  /** 自动保存的防抖时长。 */
  delayMs?: number
}

export function hasDraft(key: string): boolean {
  if (!key) return false
  return !!storage()?.getItem(keyOf(key))
}

/**
 * 表单草稿自动保存：定时把编辑中的内容写入 sessionStorage，
 * 会话失效重新登录或意外刷新后仍可恢复。只存当前标签页，不落持久存储。
 */
export function useFormDraft<T>(options: FormDraftOptions<T>) {
  let timer: ReturnType<typeof setTimeout> | null = null
  // 打开弹窗时表单会被服务端数据整体覆盖，此时不该立刻产出草稿。
  // 记录打开瞬间的快照，只有内容真正变化后才落盘。
  let baseline: string | null = null

  const currentJson = () => {
    try {
      return JSON.stringify(options.snapshot())
    } catch {
      return ''
    }
  }

  const markClean = () => {
    baseline = currentJson()
  }

  const saveNow = () => {
    const key = options.key()
    if (!key) return
    const store = storage()
    if (!store) return
    try {
      const draft = options.snapshot()
      const json = JSON.stringify(draft)
      if (!options.hasContent(draft) || (baseline !== null && json === baseline)) {
        store.removeItem(keyOf(key))
        return
      }
      store.setItem(keyOf(key), json)
    } catch {
      // 隐私模式或配额不足时静默放弃：草稿是兜底能力，不应影响正常编辑
    }
  }

  const schedule = () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => {
      timer = null
      saveNow()
    }, options.delayMs ?? 800)
  }

  const restore = (key: string): boolean => {
    if (!key) return false
    const raw = storage()?.getItem(keyOf(key))
    if (!raw) return false
    try {
      options.apply(JSON.parse(raw) as T)
      return true
    } catch {
      return false
    }
  }

  const clear = (key?: string) => {
    const target = key ?? options.key()
    if (target) storage()?.removeItem(keyOf(target))
  }

  watch(options.snapshot, schedule, { deep: true })

  onBeforeUnmount(() => {
    if (timer) {
      clearTimeout(timer)
      timer = null
      saveNow()
    }
  })

  return { restore, clear, saveNow, markClean }
}
