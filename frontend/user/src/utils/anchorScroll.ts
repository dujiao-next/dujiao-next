/**
 * 页面内锚点定位（不跳路由、只滚动）。
 *
 * 为什么不用 `el.scrollIntoView()` 一把梭：
 * 目标区块「上方」常常有异步渲染的内容 —— 首页的推荐位、侧栏分类列表、
 * 商品计数等都要等接口返回才撑开高度，会把目标区块整体往下推。
 * 只滚一次的话，落点停在推之前的旧位置，表现为「滚是滚了，但没对准」。
 *
 * 这里改成按帧观察并校正，直到落点稳定：
 *   · 观察量用「元素在文档中的绝对 Y」（rect.top + scrollY）
 *     —— 它不随滚动改变、只在布局变化时改变，正好是我们要追踪的信号。
 *     若改用视口内的相对位置，滚动动画本身就会被误判成布局变化，导致反复重滚。
 *   · 元素还不存在时先等一段时间（组件刚挂载 / 数据未到）。
 *   · 落点同时满足「稳定」与「已到位」才收工 —— 稳定但没到位说明平滑动画还在跑。
 */

export interface ScrollToAnchorOptions {
  /**
   * 目标区块顶边距视口顶部的期望距离，用来让开 sticky 头部。
   * 传函数则每次校正时重新取值（头部高度可能随交互变化）。
   */
  offset?: number | (() => number)
  /** 连续多少帧落点不变即认为稳定（默认 4） */
  stableFrames?: number
  /** 最多观察多少帧，防止页面持续变化时永不退出（默认 120，约 2s） */
  maxFrames?: number
  /** 目标区块尚未渲染时的等待帧数（默认 60，约 1s） */
  waitFrames?: number
  /** 滚动行为；默认尊重系统的「减少动态效果」设置 */
  behavior?: ScrollBehavior
}

/** 系统是否开启了「减少动态效果」 */
const prefersReducedMotion = () =>
  typeof window.matchMedia === 'function'
  && window.matchMedia('(prefers-reduced-motion: reduce)').matches

/**
 * 平滑滚动到指定 id 的区块，并在落点稳定前持续校正。
 *
 * @returns 取消函数 —— 组件卸载时调用，避免动画帧泄漏
 */
export function scrollToAnchor(id: string, options: ScrollToAnchorOptions = {}): () => void {
  const {
    offset = 0,
    stableFrames = 4,
    maxFrames = 120,
    waitFrames = 60,
    behavior,
  } = options

  const resolveOffset = () => (typeof offset === 'function' ? offset() : offset)
  const scrollBehavior: ScrollBehavior = behavior ?? (prefersReducedMotion() ? 'auto' : 'smooth')

  let raf = 0
  let frames = 0
  let stable = 0
  let lastDocTop = Number.NaN
  let lastDistance = Number.NaN

  const step = () => {
    frames += 1

    const el = document.getElementById(id)
    if (!el) {
      // 区块可能还没渲染出来（异步组件刚挂载、或列表数据未到）—— 再等一会儿
      if (frames <= waitFrames) raf = requestAnimationFrame(step)
      return
    }

    const rect = el.getBoundingClientRect()
    const docTop = Math.round(rect.top + window.scrollY)
    const want = resolveOffset()
    const targetY = Math.max(0, docTop - want)

    /*
      「被推走」兜底。
      Vue Router 的 scrollBehavior 在没有 hash 时返回 { top: 0 }，导航确认后会
      补一次 window.scrollTo({ top: 0 })。实测它通常发生在 microtask（nextTick）、
      早于这里的第一帧 rAF，但只差几毫秒 —— 时序一旦反转，刚滚好的位置就会被
      推回顶部，而下面「只在 docTop 变化时重滚」的逻辑不会察觉（布局没变）。
      正常滚动时到目标的距离是单调收敛的，只有被外部覆盖才会突然变大，
      据此判断要不要重新瞄一次。
    */
    const distance = Math.abs(window.scrollY - targetY)
    const pushedAway = Number.isFinite(lastDistance) && distance > lastDistance + 8
    lastDistance = distance

    if (docTop !== lastDocTop || pushedAway) {
      // 布局变了 / 被推走（首次进入也算）→ 重新瞄准
      lastDocTop = docTop
      stable = 0
      window.scrollTo({ top: targetY, behavior: scrollBehavior })
    } else {
      stable += 1
    }

    const arrived = Math.abs(rect.top - want) <= 2
    if ((stable >= stableFrames && arrived) || frames >= maxFrames) return

    raf = requestAnimationFrame(step)
  }

  raf = requestAnimationFrame(step)
  return () => cancelAnimationFrame(raf)
}
