/**
 * 同步 ember 模板的 i18n 命名空间到三个语言文件。
 *
 * 幂等：已存在 "ember" 块时整块替换，不存在时插入到 "vault" 之后。
 * 保留原有 CRLF 换行与 2 空格缩进；写入前先 JSON.parse 校验，避免写坏语言文件。
 */
import { readFile, writeFile } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const DIR = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src', 'i18n', 'locales');

const NS = {
  'zh-CN': {
    hero: {
      badge: '数字商品 · 即时交付',
      titleLead: '挑好商品，付款，',
      titleHighlight: '立刻拿到。',
      lede: '无需注册即可下单。支付完成后系统自动发货，卡密与订单状态随时可查。',
      ctaPrimary: '开始选购',
      ctaSecondary: '查询我的订单',
      trustAuto: '支付后自动发货',
      trustGuest: '支持游客下单',
      trustTrace: '订单永久可查',
      hotTitle: '热销商品',
      hotMore: '查看全部',
      hotEmpty: '暂无商品',
    },
    featured: {
      title: '全部商品',
      subtitle: '价格与库存均为实时数据，点击卡片查看详情。',
      empty: '商店还没有上架商品',
      emptyHint: '请先在后台「商品管理」中创建商品，这里会自动展示。',
    },
    categories: {
      title: '按分类逛',
      all: '全部分类',
    },
    latest: {
      title: '最新动态',
    },
    features: {
      autoTitle: '自动发货',
      autoBody: '支付成功后系统立即交付卡密，无需等待人工处理。',
      secureTitle: '支付安全',
      secureBody: '对接主流支付渠道，交易全程加密，订单状态可随时复核。',
      traceTitle: '订单可查',
      traceBody: '凭邮箱与订单密码即可查询历史订单与已交付的卡密。',
      multiTitle: '多种支付',
      multiBody: '支持微信、支付宝、加密货币等渠道，下单时自由选择。',
    },
    footer: {
      tagline: '数字商品即时交付。下单后自动发货，订单与卡密可随时查询。',
      shop: '商店',
      account: '账户',
      support: '支持',
      builtOn: '基于 Dujiao-Next 构建',
    },
    cart: {
      count: '共 {count} 件',
      empty: '购物车还是空的',
      emptyHint: '挑一件商品加入购物车，随时可以结算。',
      browse: '去逛逛',
      subtotal: '小计',
      checkout: '去结算',
      shippingNote: '运费与优惠将在结算页计算',
    },
    common: {
      copy: '复制',
    },
    guestOrders: {
      emailLabel: '邮箱',
      passwordLabel: '订单密码',
      resultCount: '共 {count} 笔订单',
    },
    product: {
      memberPrice: '会员价',
    },
    notFound: {
      title: '这个页面不见了',
      desc: '你访问的地址不存在，或者内容已经被移动。',
      back: '回到首页',
    },
  },

  'zh-TW': {
    hero: {
      badge: '數位商品 · 即時交付',
      titleLead: '挑好商品，付款，',
      titleHighlight: '立刻拿到。',
      lede: '無需註冊即可下單。付款完成後系統自動發貨，卡密與訂單狀態隨時可查。',
      ctaPrimary: '開始選購',
      ctaSecondary: '查詢我的訂單',
      trustAuto: '付款後自動發貨',
      trustGuest: '支援訪客下單',
      trustTrace: '訂單永久可查',
      hotTitle: '熱銷商品',
      hotMore: '查看全部',
      hotEmpty: '暫無商品',
    },
    featured: {
      title: '全部商品',
      subtitle: '價格與庫存皆為即時資料，點擊卡片查看詳情。',
      empty: '商店尚未上架商品',
      emptyHint: '請先在後台「商品管理」建立商品，這裡會自動顯示。',
    },
    categories: {
      title: '依分類逛',
      all: '全部分類',
    },
    latest: {
      title: '最新動態',
    },
    features: {
      autoTitle: '自動發貨',
      autoBody: '付款成功後系統立即交付卡密，無需等待人工處理。',
      secureTitle: '付款安全',
      secureBody: '串接主流付款管道，交易全程加密，訂單狀態可隨時複核。',
      traceTitle: '訂單可查',
      traceBody: '憑信箱與訂單密碼即可查詢歷史訂單與已交付的卡密。',
      multiTitle: '多種付款',
      multiBody: '支援微信、支付寶、加密貨幣等管道，下單時自由選擇。',
    },
    footer: {
      tagline: '數位商品即時交付。下單後自動發貨，訂單與卡密可隨時查詢。',
      shop: '商店',
      account: '帳戶',
      support: '支援',
      builtOn: '基於 Dujiao-Next 建置',
    },
    cart: {
      count: '共 {count} 件',
      empty: '購物車還是空的',
      emptyHint: '挑一件商品加入購物車，隨時可以結算。',
      browse: '去逛逛',
      subtotal: '小計',
      checkout: '去結算',
      shippingNote: '運費與優惠將在結算頁計算',
    },
    common: {
      copy: '複製',
    },
    guestOrders: {
      emailLabel: '信箱',
      passwordLabel: '訂單密碼',
      resultCount: '共 {count} 筆訂單',
    },
    product: {
      memberPrice: '會員價',
    },
    notFound: {
      title: '這個頁面不見了',
      desc: '你訪問的網址不存在，或內容已經被移動。',
      back: '回到首頁',
    },
  },

  'en-US': {
    hero: {
      badge: 'Digital goods · Instant delivery',
      titleLead: 'Pick what you need, pay, ',
      titleHighlight: 'get it instantly.',
      lede: 'No account required. Delivery is automatic once payment completes, and your keys and order status stay available at any time.',
      ctaPrimary: 'Start shopping',
      ctaSecondary: 'Find my order',
      trustAuto: 'Automatic delivery',
      trustGuest: 'Guest checkout',
      trustTrace: 'Orders always retrievable',
      hotTitle: 'Best sellers',
      hotMore: 'View all',
      hotEmpty: 'No products yet',
    },
    featured: {
      title: 'All products',
      subtitle: 'Prices and stock are live. Open a card for details.',
      empty: 'No products in this store yet',
      emptyHint: 'Create a product in the admin panel and it will show up here automatically.',
    },
    categories: {
      title: 'Browse by category',
      all: 'All categories',
    },
    latest: {
      title: 'Latest updates',
    },
    features: {
      autoTitle: 'Automatic delivery',
      autoBody: 'Keys are issued the moment payment succeeds — no manual step.',
      secureTitle: 'Secure payment',
      secureBody: 'Connected to mainstream payment channels with encrypted transactions and reviewable order status.',
      traceTitle: 'Orders you can find',
      traceBody: 'Look up past orders and delivered keys with your email and order password.',
      multiTitle: 'Multiple methods',
      multiBody: 'Pay by card, wallet or crypto — choose at checkout.',
    },
    footer: {
      tagline: 'Digital goods, delivered instantly. Orders and keys stay retrievable.',
      shop: 'Shop',
      account: 'Account',
      support: 'Support',
      builtOn: 'Built on Dujiao-Next',
    },
    cart: {
      count: '{count} items',
      empty: 'Your cart is empty',
      emptyHint: 'Add something to your cart and check out whenever you like.',
      browse: 'Browse products',
      subtotal: 'Subtotal',
      checkout: 'Checkout',
      shippingNote: 'Shipping and discounts are calculated at checkout',
    },
    common: {
      copy: 'Copy',
    },
    guestOrders: {
      emailLabel: 'Email',
      passwordLabel: 'Order password',
      resultCount: '{count} orders',
    },
    product: {
      memberPrice: 'Member price',
    },
    notFound: {
      title: 'This page is missing',
      desc: 'The address does not exist, or the content has moved.',
      back: 'Back to home',
    },
  },
};

/** 序列化为 2 空格缩进、整体再缩进 2 空格、CRLF 换行的文本（不含最外层大括号） */
function serializeBody(value, indent = 2) {
  const pad = ' '.repeat(indent);
  const lines = JSON.stringify(value, null, 2).split('\n');
  // 首行 `{` 与末行 `}` 都剥掉，外层大括号由调用方补
  return lines.slice(1, -1).map((line) => pad + line).join('\r\n');
}

const START = '  "ember": {';
const END = '\r\n  },\r\n';
const ANCHOR = '  "breadcrumb": {';

for (const [locale, ns] of Object.entries(NS)) {
  const path = join(DIR, `${locale}.json`);
  const raw = await readFile(path, 'utf8');

  const block = `${START}\r\n${serializeBody(ns)}\r\n  },\r\n`;

  let next;
  const startIndex = raw.indexOf(`\r\n${START}`);

  if (startIndex !== -1) {
    // 已存在：整块替换。块的结束是第一个 2 空格缩进的 `},`
    const bodyStart = startIndex + 2;
    const endIndex = raw.indexOf(END, bodyStart);
    if (endIndex === -1) throw new Error(`${locale}.json 中 ember 块结构异常，找不到结束标记`);
    next = raw.slice(0, bodyStart) + block + raw.slice(endIndex + END.length);
  } else {
    const anchorIndex = raw.indexOf(ANCHOR);
    if (anchorIndex === -1) throw new Error(`${locale}.json 中找不到锚点 ${ANCHOR}`);
    next = raw.slice(0, anchorIndex) + block + '\r\n' + raw.slice(anchorIndex);
  }

  // 先校验再落盘
  const parsed = JSON.parse(next);
  if (!parsed.ember) throw new Error(`${locale}.json 写入后缺少 ember 命名空间`);

  await writeFile(path, next, 'utf8');
  process.stdout.write(`done ${locale}（${Object.keys(ns).length} 个分组，${next.length - raw.length >= 0 ? '+' : ''}${next.length - raw.length} 字节）\n`);
}
