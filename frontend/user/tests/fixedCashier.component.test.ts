import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { parse, compileScript } from 'vue/compiler-sfc'
import ts from 'typescript'
import { createRenderer, h, nextTick, reactive } from 'vue'
import QRCode from 'qrcode'

// Mount the actual SFC with Vue's renderer. Only browser IO and UI-library Button
// are replaced; watchers, rendering branches, timers and request handling run.
const source = await readFile(new URL('../src/components/payment/FixedUsdtCashier.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
let compiled = compileScript(descriptor, { id: 'cashier-test', inlineTemplate: true }).content
compiled = compiled
  .replace(/import \{ useI18n \} from ['"]vue-i18n['"]/, "const useI18n = () => ({ t: (key: string) => key })")
  .replace(/import QRCode from ['"]qrcode['"]/, 'const QRCode = globalThis.__cashierTest.qr')
  .replace(/import \{ Button \} from ['"]@\/components\/ui\/button['"]/, 'const Button = globalThis.__cashierTest.Button')
  .replace(/import \{ paymentAPI \} from ['"]\.\.\/\.\.\/api['"]/, 'const paymentAPI = globalThis.__cashierTest.api')
  .replace(/import \{ copyText \} from ['"]\.\.\/\.\.\/utils\/clipboard['"]/, 'const copyText = globalThis.__cashierTest.copy')
  .replace(/from ['"]vue['"]/g, `from '${import.meta.resolve('vue')}'`)
  .replace(/from ['"]\.\.\/\.\.\/utils\/fixedCashier['"]/, `from '${new URL('../src/utils/fixedCashier.ts', import.meta.url).href}'`)
const moduleCode = ts.transpileModule(compiled, { compilerOptions: { target: ts.ScriptTarget.ESNext, module: ts.ModuleKind.ESNext } }).outputText

const node = (tag = '', text = '') => ({ tag, text, props: {} as Record<string, any>, children: [] as any[], parent: null as any })
const renderer = createRenderer<any, any>({
  createElement: tag => node(tag), createText: text => node('#text', text), createComment: text => node('#comment', text),
  setText: (el, text) => { el.text = text }, setElementText: (el, text) => { el.text = text; el.children = [] },
  parentNode: el => el.parent, nextSibling: el => el.parent?.children[el.parent.children.indexOf(el) + 1] ?? null,
  patchProp: (el, key, _prev, next) => { el.props[key] = next },
  insert: (el, parent, anchor) => {
    if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1)
    el.parent = parent
    const at = anchor ? parent.children.indexOf(anchor) : -1
    if (at < 0) parent.children.push(el); else parent.children.splice(at, 0, el)
  },
  remove: el => { if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1); el.parent = null },
})
const descendants = (el: any): any[] => [el, ...el.children.flatMap(descendants)]
const textOf = (el: any): string => descendants(el).filter(e => e.tag !== '#comment').map(e => e.text).join(' ')
const tick = async () => { await Promise.resolve(); await nextTick(); await Promise.resolve(); await nextTick() }
function deferred() { let resolve!: (value: any) => void; let reject!: (error: any) => void; const promise = new Promise<any>((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject } }
const quote = (id = 7, network = 'binance') => ({ payment_id: id, state: 'ready', server_time_ms: 1000000, display_until_ms: 1015000,
  details: { network, token: 'USDT', contract: network === 'tron' ? 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t' : '0x55d398326f99059ff775485246999027b3197955',
    recipient: network === 'tron' ? 'TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdV' : '0x1111111111111111111111111111111111111111',
    raw_amount: network === 'tron' ? '14000100' : '14000100000000000000', decimals: network === 'tron' ? 6 : 18, amount: '14.0001',
    fiat_amount: '100.00', fiat_currency: 'CNY', expires_at_ms: 1600000 } })

test('mounted cashier rejects stale IO, hides unsafe instructions and copies exact values', async t => {
  const doc = Object.assign(new EventTarget(), { hidden: false })
  const win = new EventTarget()
  const originals = new Map<string, PropertyDescriptor | undefined>()
  for (const [key, value] of Object.entries({ document: doc, window: win, navigator: { onLine: true } })) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key))
    Object.defineProperty(globalThis, key, { configurable: true, value })
  }
  t.after(() => { for (const [key, old] of originals) { if (old) Object.defineProperty(globalThis, key, old); else delete (globalThis as any)[key] }; delete (globalThis as any).__cashierTest })
  let now = 100
  t.mock.method(performance, 'now', () => now)
  const timers = new Map<number, () => void>()
  t.mock.method(globalThis, 'setInterval', (fn: () => void, ms: number) => { timers.set(ms, fn); return ms })
  t.mock.method(globalThis, 'clearInterval', (id: number) => { timers.delete(id) })
  const requests: any[] = [], qrRequests: any[] = [], copies: string[] = []
  const environment = {
    api: { cashier: (query: any) => { const pending = deferred(); requests.push({ query, ...pending }); return pending.promise } },
    qr: { toDataURL: (address: string, options: any) => { const pending = deferred(); qrRequests.push({ address, options, ...pending }); return pending.promise } },
    copy: async (value: string) => { copies.push(value) },
    Button: { setup: (_: any, ctx: any) => () => h('button', ctx.attrs, ctx.slots.default?.()) },
  }
  ;(globalThis as any).__cashierTest = environment
  const { default: Cashier } = await import(`data:text/javascript;base64,${Buffer.from(moduleCode).toString('base64')}`)
  const props = reactive({ paymentId: 7, orderNo: 'first' })
  let closed = 0
  const app = renderer.createApp({ setup: () => () => h(Cashier, { ...props, onRefreshOrder: () => { closed++ } }) })
  const root = node('root'); app.mount(root); t.after(() => app.unmount())
  const lastRequest = () => requests.at(-1)
  const answer = async (data: any) => { lastRequest().resolve({ data: { data } }); await tick() }
  const images = () => descendants(root).filter(e => e.tag === 'img')
  const hidden = () => { assert.doesNotMatch(textOf(root), /14\.0001|0x111111|TBXSw/); assert.equal(images().length, 0) }
  const click = async (label: string) => { const btn = descendants(root).find(e => e.tag === 'button' && textOf(e).includes(label)); assert.ok(btn, label); await btn.props.onClick(); await tick() }
  assert.deepEqual(lastRequest().query, { order_no: 'first', payment_id: 7 })
  await answer(quote())
  assert.match(textOf(root), /14\.0001 USDT/)
  const qr = qrRequests.at(-1)
  assert.equal(qr.address, quote().details.recipient)
  const actualPNG = await QRCode.toDataURL(qr.address, qr.options)
  assert.match(actualPNG, /^data:image\/png;base64,/)
  qr.resolve(actualPNG); await tick(); assert.equal(images()[0].props.src, actualPNG)
  await click('payment.fixed.copyAmount'); await click('payment.copyWalletAddress')
  assert.deepEqual(copies, ['14.0001', quote().details.recipient])

  // An in-flight refresh cannot restore details after going offline.
  timers.get(5000)!(); const beforeOffline = lastRequest()
  Object.assign(navigator, { onLine: false }); win.dispatchEvent(new Event('offline')); await tick(); hidden()
  beforeOffline.resolve({ data: { data: quote() } }); await tick(); hidden()
  Object.assign(navigator, { onLine: true }); win.dispatchEvent(new Event('online')); await tick()
  await answer(quote(7, 'tron')); assert.match(textOf(root), /TRC20/)
  const oldQR = qrRequests.at(-1)

  // Freeze removes both address and QR; delayed QR completion stays hidden.
  timers.get(5000)!(); await answer({ ...quote(), state: 'paused' }); hidden()
  oldQR.resolve(actualPNG); await tick(); hidden()
  assert.match(textOf(root), /states.paused/)

  // Route change invalidates the previous order's pending request.
  timers.get(5000)!(); const oldOrder = lastRequest()
  props.orderNo = 'second'; props.paymentId = 8; await tick(); hidden()
  assert.deepEqual(lastRequest().query, { order_no: 'second', payment_id: 8 })
  oldOrder.resolve({ data: { data: quote() } }); await tick(); hidden()
  await answer(quote(8)); assert.match(textOf(root), /14\.0001/)

  doc.hidden = true; doc.dispatchEvent(new Event('visibilitychange')); await tick(); hidden()
  doc.hidden = false; doc.dispatchEvent(new Event('visibilitychange')); await tick(); hidden()
  await answer(quote(8)); assert.match(textOf(root), /14\.0001/)

  // Copy checks live monotonic time even before the render timer has fired.
  now += 15001; await click('payment.fixed.copyAmount'); hidden(); assert.equal(copies.length, 2)
  timers.get(5000)!(); const slow = lastRequest(); now += 15001
  slow.resolve({ data: { data: quote(8) } }); await tick(); hidden()
  timers.get(5000)!(); await answer(quote(8)); now += 15001; timers.get(250)!(); await tick(); hidden()
  timers.get(5000)!(); lastRequest().reject(new Error('synthetic network error')); await tick(); hidden()
  timers.get(5000)!(); await answer({ ...quote(8), state: 'closed' }); hidden(); assert.equal(closed, 1)

  timers.get(5000)!(); const onUnmount = lastRequest()
  app.unmount(); onUnmount.resolve({ data: { data: quote(8) } }); await tick()
  assert.equal(timers.size, 0); assert.equal(root.children.length, 0)
})
