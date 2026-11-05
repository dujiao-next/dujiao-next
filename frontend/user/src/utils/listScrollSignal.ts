/**
 * 列表页滚动恢复信号。
 *
 * useProductList 在数据加载完成后 resolve 此 Promise，
 * router 的 scrollBehavior 在后退到列表页时 await 它，
 * 确保页面高度撑开后再恢复 savedPosition，避免截断。
 *
 * 每次 loadProducts 开始前通过 resetListScrollSignal() 重置。
 */

let _resolve: (() => void) | null = null
let _promise: Promise<void> = Promise.resolve()

export function resetListScrollSignal() {
  _promise = new Promise<void>((resolve) => {
    _resolve = resolve
  })
}

export function resolveListScrollSignal() {
  _resolve?.()
  _resolve = null
}

export function waitForListScrollSignal(timeoutMs = 2000): Promise<void> {
  return Promise.race([
    _promise,
    new Promise<void>((resolve) => setTimeout(resolve, timeoutMs)),
  ])
}
