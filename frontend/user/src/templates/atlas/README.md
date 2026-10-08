# Atlas — 参考站复刻模板

复刻对象：`https://accboy7flybird.acceboy.com/`（下文称「参考站」）。
用途：个人测试。**只复刻版式与视觉风格，不复刻内容** —— 文案、图片、品牌名、
字体文件均不取自参考站，模板渲染的是本站自己的商品与分类数据。

---

## 一、视觉风格的来源（可核对，非目测）

令牌抽取自参考站 SSR HTML 内联的 styled-components CSS（`data-styled` 的
语义名→哈希映射可反查）。**不是目测，也不是从 `:root` 变量猜的** ——
参考站的 `:root` 只有 10 个全局变量，真正的版式数值全在各组件的规则里：

| 参考站组件 | 关键数值 | 映射到 atlas |
|---|---|---|
| `layout-header__StyledHeader` | `height:72px; background:#050505` | `.atlas-header` |
| `…StyledLayoutHeaderContent` | `width:1340px; margin:0 auto` | `--shell: 1340px` |
| `…StyledSearchBox` | `flex:1; max-width:485px; background:#fff; height:40px; border-radius:5px` | `.atlas-search` |
| `…StyledLanguage` | `font-size:14px; color:#fff; margin-right:16px` | `.atlas-lang` |
| `…StyledCooperate` | `height:36px`；图标 `fill:#d49c0a`；hover 填金 | `.atlas-cooperate` |
| `…StyledLoginBtn` | hover `border-color:#fff; background:rgba(255,255,255,.1)` | `.atlas-ghost-btn` |
| `…StyledPcHeaderAd` | `height:40px; background:#050505; border-bottom:2px solid #e60027` | `.atlas-navbar` |
| `…pc-header-ad-item` | `height:34px; border-radius:8px 8px 0 0; padding:0 20px; font-size:14px; color:rgba(255,255,255,.85)` | `.atlas-navbar__item` |
| `…pc-header-ad-item.has-url` | `color:#5ff8ff; font-weight:700` | `.atlas-navbar__item--hot` |
| `home__StyledTop` | `background:#ebebf1; padding:16px 0`；`.icon 40x40` | `.atlas-trustbar` |
| `home-banner__StyledHomeBanner` | `width:1340px; height:70px; border-radius:12px` | `.atlas-banner` |
| `home__StyledSidebar` | `width:240px; background:#fff; border-radius:14px; padding:18px 20px; box-shadow:2px 2px 8px 0 rgba(43,74,129,.1)` | `.atlas-sidebar` |
| `…sidebar row` | `padding:12px 16px; border-radius:10px; font-size:14px`；hover `rgba(230,0,39,.1)`；激活 `#e60027` | `.atlas-sidebar__row` |
| `home__StyledMain` | `width:1080px` | `.atlas-main-col` |
| `home__StyledRecommendList` | `.recommend-top` grid `700px 335px` gap `30px`；item `border-radius:12px`、`aspect-ratio:335/178` | `.atlas-reco-*` |
| `home__StyledRecommendModule` | `.module-content width:1340px; background:#f3f0ef; border-radius:14px; padding:30px 25px`；`.module-title font-size:26px/700`；`.module-list repeat(4,1fr) gap:27px 26px` | `.atlas-module*` |
| `download-float__StyledPageDownFloat` | `width:1340px; position:fixed; bottom:0`；`.btn min-width:183px; height:40px; border-radius:28px` | `.atlas-appfloat` |
| `layout__StyledCustomerFloat` | `position:fixed; bottom:-8px; right:0`；`.box background:#e60027`；`.box2 linear-gradient(134deg,#4f545d,#131321)` | `.atlas-custfloat` |

**四个决定观感、也最容易被做错的点：**

1. **整条顶栏是深色 `#050505`，不是白底。** 顶栏分两层：72px 主行（品牌 + 白色
   搜索框 + 右侧操作）+ 40px 分类导航条，导航条底部有一条 **2px 红 `#e60027` 下边框**。
   把它做成白底是上一版最大的错。
2. **主 CTA 是红 `#e60027`，不是黑。** 红色用于：搜索按钮、导航条激活项、
   侧栏「推荐」表头、HOT 标、右侧客服。金色 `#d49c0a` 专给「推广合作」。
   青色 `#5ff8ff` 专给导航条末尾那个带图标的特殊入口。
3. **版心 1340px。** 主区是 **240px 侧栏 + 1080px 内容**（含 20px 间隙正好 1340），
   推荐位是 `700px + 335px` 的 grid，下排 3×335px。用 1200px 版心 + 全宽宫格是错的。
4. **字体是 DIN 系。** D-DIN-PRO 是参考站自托管文件，**不引外部资源**；
   改用「DIN 观感」的本地字体栈：`Bahnschrift`（Windows 自带）→ `DIN Alternate`
   （macOS 自带）→ Inter（项目已有依赖）→ 系统 UI。不新增任何依赖。

## 二、复刻范围：页面与模块清单

参考站的页面形态（从其 SSR HTML 与链接结构归纳）：

| 参考站路径 | 形态 | 对应本项目页面 |
|---|---|---|
| `/` | 首页 | `Home.vue` |
| `/categories/:slug` | 商品列表（左侧分类栏 + 网格） | `Products.vue` |
| `/products/:slug` | 商品详情（规格选择 + 立即购买） | **回退 classic** `views/ProductDetail.vue` |
| `/cart`、`/checkout` | 购物车 / 结算 | `Cart.vue`、`Checkout.vue` |
| `/news`、`/news-detail/:id` | 新闻列表 / 详情 | `Blog.vue`、`BlogDetail.vue` |
| `/info/*` | 关于、帮助、退换、隐私等 | `About.vue`、`Legal.vue` |
| `/affiliate` | 推广合作 | `PersonalCenter.vue` 下的分销面板 |
| `/guest/orders` | 订单查询 | `GuestOrders.vue` |

### 模块级拆解

**首页**（参考站自上而下）

1. 顶栏工具条：语言/币种 · 推广合作 · 订单查询 · 登录/注册
2. 主导航：品牌 + 一级分类直链（参考站铺了 8 个）+ 购物车
3. Hero：品牌大字 + 一行副标题 + 三个信任徽章（即时交付 / 被广泛信赖 / 支付安全）
4. 分类导航：一排带方形图标的入口
5. 商品分区 ×N：分区头（左标题+副标题，右「发现更多」）+ 商品网格
6. 新闻&促销：卡片列表 + 查看更多
7. 用户评价：轮播 + 分页
8. 页脚：纯黑，多列链接 + 在线客服

**列表页**：面包屑 → 左侧分类栏（一级 + 二级）→ 品类标题 → 商品网格 → 分页

**详情页**：**刻意沿用 classic 默认页**（`views/ProductDetail.vue`），不由本模板覆盖。
配色通过下方「经典页面配色对齐」的 `--ui-*` 重映射自动跟随 atlas，无需另写样式。

### 本轮实现状态

| 文件 | 状态 |
|---|---|
| `layout/AtlasLayout.vue` | ✅ 已实现（含顶栏工具条 + 主导航 + 纯黑页脚） |
| `Home.vue` | ✅ 已实现（Hero + 信任徽章 + 分类导航 + 商品分区） |
| `Products.vue` | ✅ 已实现（面包屑 + 分类栏 + 网格 + 分页） |
| `components/AtlasProductCard.vue` | ✅ |
| `components/AtlasSectionHead.vue` | ✅ |
| `ProductDetail.vue` / `Cart.vue` / `Checkout.vue` | ⏳ 未覆盖 → **自动回退 classic** |
| 其余页面 | ⏳ 未覆盖 → 自动回退 classic |

未覆盖的页面**不需要任何额外配置**：`templateView()` 在模板目录里找不到同名文件时
自动用 classic 的 loader（见第三节）。所以本模板当前是一个「首页 + 列表页已复刻、
其余沿用 classic」的可用状态，而不是半成品。

> 商品详情页是**刻意退回 classic 默认页**的，不是漏做：它是交易链路的一环
> （规格 → 下单 → 支付），复用久经使用、已端到端验证过的默认页，比复刻版更稳。
> 配色靠 `--ui-*` 重映射跟随本模板，视觉上仍是 atlas 的调色板。
> 若将来要改成复刻版，只需新建 `ProductDetail.vue`，无需改任何注册代码。

## 三、目录结构

```
src/templates/atlas/
├── styles/
│   └── atlas.css                 # 设计令牌 + 复用件，作用域限定 .atlas-scope
├── layout/
│   └── AtlasLayout.vue           # 外壳：顶栏工具条 + 主导航 + 纯黑页脚
├── components/
│   ├── AtlasProductCard.vue      # 商品卡（首页 / 列表页共用）
│   └── AtlasSectionHead.vue      # 分区头（左标题 + 右「发现更多」）
├── Home.vue                      # 首页
├── Products.vue                  # 商品列表
└── README.md                     # 本文
```

命名约定（沿用 vault 的既有惯例）：
- 模板根目录 = 页面，文件名与 `src/views/` 同名（`Home.vue` ↔ `views/Home.vue`）
- 私有组件加模板前缀（`Atlas*`），避免与其他模板的组件重名
- 令牌文件放 `styles/<name>.css`，只导出变量与少量复用类

## 四、接入方式（4 处，缺一不可）

| # | 文件 | 改动 |
|---|---|---|
| 1 | `src/templates/registry.ts` | `StorefrontTemplate` 联合类型 + `STOREFRONT_TEMPLATES` 数组各加 `'atlas'`；`templateViews` 加一行 `atlas: import.meta.glob('./atlas/**/*.vue')` |
| 2 | `src/App.vue` | `defineAsyncComponent` 引入 `AtlasLayout`，并在 `templateShell` 里加一个分支 |
| 3 | `internal/constants/constants.go` | 新增 `StorefrontTemplateAtlas = "atlas"` |
| 4 | `internal/modules/settings/application/site_normalize.go` | `normalizeStorefrontTemplate()` 的 switch 放行 `atlas`（否则后台保存会被静默改回 classic） |

> ⚠️ 第 3、4 条是**后端**改动。只改前端的话，`?template=atlas` 本地预览可用，
> 但后台正式切换会被归一化白名单拦掉。
>
> 前端还有两处需要同步：`registry.ts` 的 `import.meta.glob` **参数必须是静态字面量**，
> 所以每新增一个模板都要手写一行；`App.vue` 的外壳映射同理，无法自动发现。

### 怎么用

```
# 本地预览（写进 localStorage，?template=reset 清除）
http://localhost:5173/?template=atlas

# 正式启用
后台 → 设置 → 站点与主题 → 前台模板 → 选「Atlas」
```

## 五、核心交互（需要保留的）

| 交互 | 实现位置 | 说明 |
|---|---|---|
| 顶部分类按钮 | `AtlasLayout` `goCategory()` | **页内锚点定位**：原地筛选（只改 `query.category`）+ 平滑滚到 `#atlas-catalog`，不跳 `/categories/:slug` |
| 「立即选购」banner | `Home.vue` `goShopNow()` | **页内锚点定位**：滚到 `#atlas-products`，不改地址、不动筛选状态 |
| 顶部「首页」按钮 | `AtlasLayout` `goHome()` | 回到页面顶部 |
| 顶部「HOT」按钮 | `AtlasLayout` `goHotProducts()` | 滚到 `#atlas-products` |
| 顶部「购物车」按钮 | `AtlasLayout` | 保持跳转 `/cart` —— 它是独立页面，页内没有对应区块 |
| 分类切换（列表页侧栏） | `Products.vue` `selectCategory()` | 写入 `/categories/:slug`，可分享可刷新；并重置到第 1 页 |
| 分页 | `Products.vue` `changePage()` | 切页回到顶部（`behavior: 'smooth'`） |
| 商品卡 → 详情 | `AtlasProductCard` emit `click` → `router.push('/products/:slug')` | 整卡可点 |
| 语言切换 | `AtlasLayout` | 走 `appStore.setLocale()`，与 classic 同一状态源 |
| 购物车角标 | `AtlasLayout` | 读 `cartStore.totalItems`，实时更新 |
| 浮层点击外部关闭 | `AtlasLayout` `onDocClick` | 语言 / 移动端菜单 |
| Teleport 浮层配色 | `AtlasLayout` 挂 `body.atlas-tokens` | Toast / ConfirmDialog / Select 下拉会 Teleport 到 body，不挂这个 class 就会退回 classic 调色板 |

### 页内锚点定位（顶部导航 / 立即选购）

锚点 id 定义在 `anchor.ts`，布局层（`AtlasLayout`）与页面层（`Home.vue`）共用同一份，
避免两边写散、改一处漏一处：

| 锚点 | 挂在哪个元素 | 为什么是它 |
|---|---|---|
| `#atlas-catalog` | 三栏主区外层 `.atlas-shell` | 上方（信任徽章 + banner）高度固定，落点稳；侧栏本身会随筛选变高变矮 |
| `#atlas-products` | 商品模块（未筛选是整宽分区，已筛选是主列里的紧凑网格） | 两处挂同一个 id，但同一时刻只渲染一个，不冲突 |

滚动由 `utils/anchorScroll.ts` 的 `scrollToAnchor()` 负责，不是一句
`scrollIntoView()`：

- **按帧校正**：目标区块上方常有异步渲染的内容（推荐位、分类列表），接口返回后会把
  区块整体下推，只滚一次会停在推之前的旧位置。这里用「元素在文档中的绝对 Y」
  （`rect.top + scrollY`，不随滚动变、只在布局变时变）判断落点是否被推走并重新瞄准。
- **让开 sticky 公告条**：`atlasStickyOffset()` 实时量 `.atlas-notice` 的高度当偏移，
  公告条被关掉后自动变 0，不会白留一截。
- **被推走兜底**：Vue Router 的 `scrollBehavior` 在没有 hash 时返回 `{ top: 0 }`，
  导航后会补一次 `window.scrollTo({ top: 0 })`。实测它通常早于本函数的第一帧
  （microtask vs rAF，相差约 7ms），但时序一旦反转就会把落点推回顶部 ——
  所以额外检测「到目标的距离突然变大」并重新瞄准。
- **跨页情况**：导航条在每个页面都渲染，但锚点只在首页。当前页面没有该锚点时
  先 `router.push('/')`，`await` 导航完成后再滚；`scrollToAnchor` 内部还会等
  元素出现（`waitFrames`）并校正落点。

实测（无头 Chrome + CDP，9 个场景全部命中，落点误差 0px）：未筛选/已筛选 ×
分类按钮/立即选购/HOT、公告条开与关、详情页跨页返回、购物车跳转。


**参考站有、但本站刻意不做的交互**：它的详情页只有「立即购买」、没有购物车
（直购流程）。本站保留购物车 —— 这是项目既有能力，不能为了像参考站而砍掉。
版式上仍然照参考站做，只是 CTA 旁多一个「加入购物车」。

## 六、响应式适配要求

| 断点 | 要求 |
|---|---|
| `< 640px` | 商品网格 **2 列**；分类导航 3 列；信任徽章改单列堆叠；隐藏顶栏工具条的语言项，收进移动端菜单 |
| `640–1023px` | 网格 3 列；分类导航 5 列；列表页侧栏改为横向整宽（不吸顶） |
| `≥ 1024px` | 网格 4 列；分类导航 10 列；列表页侧栏 208px 宽 + `sticky`（`top: 80px`，避开吸顶导航） |
| `≥ 1024px` | 主导航展开一级分类直链；`< 1024px` 折叠进汉堡菜单 |

其他约束：
- 版心统一 `max-width: 1200px`，内边距 `16px`（`--gap-inline`）
- 商品卡图片区固定 `aspect-ratio: 4/3`，用 `object-fit: contain`（商品图比例不统一，
  用 `cover` 会裁掉主体）
- 价格用 `font-variant-numeric: tabular-nums`，避免数字宽度跳动
- 所有图片 `loading="lazy" decoding="async"`

## 七、不影响现有模板的机制

**1. 样式隔离**：`atlas.css` 的所有选择器都限定在 `.atlas-scope` 或
`body.atlas-tokens` 下。`body.atlas-tokens` 由 `AtlasLayout` 挂载时打上、
`onUnmounted` 时摘掉，所以只有 atlas 激活期间才生效。

**2. 页面级自动回退**：`templateView(name, classicLoader)` 在当前模板目录里找不到
`<name>.vue` 时，直接返回 classic 的 loader。因此 atlas 缺哪些页面，那些页面就
继续用 classic —— 不会白屏，也不需要占位文件。

**3. 按需加载**：`import.meta.glob` 是懒加载，`AtlasLayout` 走
`defineAsyncComponent`。选 classic 的用户不会下载 atlas 的任何 chunk 或样式。

**4. 后端白名单是「加一项」而不是「改语义」**：`normalizeStorefrontTemplate()`
的 `default` 分支仍然返回 classic，未知值的行为不变。

### 验证方式

```bash
# 容器内
bash .devcontainer/doctor.sh          # 期望：失败 0
cd frontend/user && pnpm exec vue-tsc -b   # 期望：TSC_EXIT=0

# 逐个模板切换，确认互不影响
http://localhost:5173/?template=classic
http://localhost:5173/?template=vault
http://localhost:5173/?template=atlas
http://localhost:5173/?template=reset
```

## 八、后续补页面的做法

新建 `src/templates/atlas/<页面名>.vue`（文件名必须与 `views/` 下同名），
**不需要改任何注册代码** —— `import.meta.glob` 会在下次 dev server 重启后自动收录。

> 注意：`import.meta.glob` 的结果在 transform 阶段求值并被缓存，
> **新增模板目录后需要重启 dev server**（加页面文件通常不用，但新目录要）。
