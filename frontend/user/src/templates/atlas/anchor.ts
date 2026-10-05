/**
 * atlas 模板的页内锚点定义。
 *
 * 为什么集中在一处：顶部分类按钮在布局层（AtlasLayout.vue），目标区块在页面层
 * （Home.vue），两边必须用同一个 id。写散在两个文件里，改一处漏一处就会变成
 * 「点了没反应」而且很难查。
 */

export const ATLAS_ANCHOR = {
  /** 分类区：三栏主区（含左侧分类栏）。两种筛选状态下都存在，是最稳的落点。 */
  catalog: 'atlas-catalog',
  /**
   * 商品区：未筛选时是整宽的热门商品分区，已筛选时是收进主列里的紧凑网格。
   * 两处都挂同一个 id，但同一时刻只渲染一个，不会冲突。
   */
  products: 'atlas-products',
} as const

/**
 * sticky 公告条会盖住锚点顶部，滚动时要让出它的高度。
 * 公告条可以被用户关闭（v-if 移除），所以实时量渲染高度，而不是写死 56px ——
 * 否则关闭公告条后每次定位都会多留出一截空白。
 */
export const atlasStickyOffset = () =>
  document.querySelector<HTMLElement>('.atlas-notice')?.getBoundingClientRect().height ?? 0
