import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const sheetVariants = fs.readFileSync(new URL('../src/components/ui/sheet/index.ts', import.meta.url), 'utf8')

test('sheet side variants do not leave an opening transform applied', () => {
  assert.match(sheetVariants, /left: "[^"]*data-\[state=open\]:translate-x-0/)
  assert.match(sheetVariants, /right:\s*\n\s*"[^"]*data-\[state=open\]:translate-x-0/)
})
