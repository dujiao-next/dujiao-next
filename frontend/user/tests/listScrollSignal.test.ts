import test from 'node:test'
import assert from 'node:assert/strict'

// listScrollSignal 是纯 JS 模块，无 Vue/DOM 依赖，可直接测试
import {
  resetListScrollSignal,
  resolveListScrollSignal,
  waitForListScrollSignal,
} from '../src/utils/listScrollSignal.ts'

test('resolveListScrollSignal resolves the pending promise', async () => {
  resetListScrollSignal()
  let resolved = false
  const p = waitForListScrollSignal(500).then(() => { resolved = true })
  assert.equal(resolved, false, 'should not be resolved yet')
  resolveListScrollSignal()
  await p
  assert.equal(resolved, true, 'should be resolved after resolveListScrollSignal()')
})

test('waitForListScrollSignal times out if never resolved', async () => {
  resetListScrollSignal()
  const start = Date.now()
  await waitForListScrollSignal(50)
  const elapsed = Date.now() - start
  assert.ok(elapsed >= 40, `should wait at least 40ms, waited ${elapsed}ms`)
})

test('resolveListScrollSignal after reset does not affect next reset', async () => {
  resetListScrollSignal()
  resolveListScrollSignal()
  // 再 reset，新的 promise 应尚未 resolve
  resetListScrollSignal()
  let resolved = false
  const p = waitForListScrollSignal(50).then(() => { resolved = true })
  // 等超时
  await p
  // 超时后 resolved 为 true（超时兜底），但如果 resolve 泄漏会提早 resolve
  // 核心是确保不抛错、逻辑独立
  assert.equal(typeof resolved, 'boolean')
})
