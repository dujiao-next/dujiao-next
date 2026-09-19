import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const sheet = fs.readFileSync(new URL('../src/components/ui/sheet/SheetContent.vue', import.meta.url), 'utf8')

test('closed sheet overlay and content cannot intercept page taps', () => {
  assert.match(sheet, /DialogOverlay[\s\S]*?data-\[state=closed\]:pointer-events-none/)
  assert.match(sheet, /DialogContent[\s\S]*?data-\[state=closed\]:pointer-events-none/)
})
