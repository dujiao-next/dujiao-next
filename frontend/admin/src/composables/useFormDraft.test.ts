import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, defineComponent, h, nextTick, reactive, type App } from 'vue'
import { hasDraft, useFormDraft } from './useFormDraft'

interface Draft {
  title: string
  slug: string
}

let app: App | undefined

function mountDraft(key: () => string) {
  const state = reactive<Draft>({ title: '', slug: '' })
  let api!: ReturnType<typeof useFormDraft<Draft>>

  const Comp = defineComponent({
    setup() {
      api = useFormDraft<Draft>({
        key,
        snapshot: () => ({ title: state.title, slug: state.slug }),
        apply: (draft) => {
          state.title = draft.title
          state.slug = draft.slug
        },
        hasContent: (draft) => String(draft.title || '').trim() !== '',
        delayMs: 20,
      })
      return () => h('div')
    },
  })

  app = createApp(Comp)
  app.mount(document.createElement('div'))
  return { state, api }
}

const flushSave = async () => {
  await nextTick()
  vi.advanceTimersByTime(20)
}

beforeEach(() => {
  sessionStorage.clear()
  vi.useFakeTimers()
})

afterEach(() => {
  app?.unmount()
  app = undefined
  vi.useRealTimers()
})

describe('useFormDraft', () => {
  it('输入后按防抖时间写入草稿', async () => {
    const { state } = mountDraft(() => 'post-new')

    state.title = '写给自己的第一篇'
    await flushSave()

    expect(hasDraft('post-new')).toBe(true)
    const raw = sessionStorage.getItem('admin-form-draft:post-new')
    expect(JSON.parse(raw as string)).toEqual({ title: '写给自己的第一篇', slug: '' })
  })

  it('空表单不产生草稿', async () => {
    const { state } = mountDraft(() => 'post-new')

    state.slug = 'only-slug'
    await flushSave()

    expect(hasDraft('post-new')).toBe(false)
  })

  it('markClean 之后未修改不落盘（模拟打开弹窗加载服务端数据）', async () => {
    const { state, api } = mountDraft(() => 'post-7')

    state.title = '服务端数据'
    api.markClean()
    await flushSave()

    expect(hasDraft('post-7')).toBe(false)
  })

  it('markClean 之后再修改仍然会保存', async () => {
    const { state, api } = mountDraft(() => 'post-7')

    state.title = '服务端数据'
    api.markClean()
    await flushSave()

    state.title = '用户改过的标题'
    await flushSave()

    expect(hasDraft('post-7')).toBe(true)
  })

  it('内容被清空后移除草稿', async () => {
    const { state } = mountDraft(() => 'post-3')

    state.title = '临时内容'
    await flushSave()
    expect(hasDraft('post-3')).toBe(true)

    state.title = ''
    await flushSave()
    expect(hasDraft('post-3')).toBe(false)
  })

  it('restore 会把草稿写回表单', () => {
    sessionStorage.setItem(
      'admin-form-draft:post-9',
      JSON.stringify({ title: '中断前的标题', slug: 'old-slug' }),
    )
    const { state, api } = mountDraft(() => 'post-9')

    expect(api.restore('post-9')).toBe(true)
    expect(state.title).toBe('中断前的标题')
    expect(state.slug).toBe('old-slug')
  })

  it('restore 在没有草稿时返回 false 且不改动表单', () => {
    const { state, api } = mountDraft(() => 'post-11')

    expect(api.restore('post-11')).toBe(false)
    expect(state.title).toBe('')
  })

  it('clear 删除指定草稿', async () => {
    const { state, api } = mountDraft(() => 'post-5')

    state.title = '要丢弃的内容'
    await flushSave()
    expect(hasDraft('post-5')).toBe(true)

    api.clear('post-5')
    expect(hasDraft('post-5')).toBe(false)
  })

  it('key 返回空串时不保存（弹窗关闭态）', async () => {
    const { state } = mountDraft(() => '')

    state.title = '不该被保存'
    await flushSave()

    expect(hasDraft('')).toBe(false)
    expect(sessionStorage.length).toBe(0)
  })
})
