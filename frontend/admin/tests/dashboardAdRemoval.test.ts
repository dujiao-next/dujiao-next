import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const dashboard = fs.readFileSync(new URL('../src/views/Dashboard.vue', import.meta.url), 'utf8')

test('admin dashboard does not load or render sponsored advertising', () => {
  assert.doesNotMatch(dashboard, /DashboardAd/)
  assert.doesNotMatch(dashboard, /dashboard_sponsored/)
})
