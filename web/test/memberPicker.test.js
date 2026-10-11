// The picker's X button is only honest where an empty value means something
// (filters, "unassigned" owners). A document's author can never be empty: the
// editor save, the API client and the server all drop or refill it, so an X on
// that picker offered an action that could not take effect (issue #432).
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
// FIRST, before any import that reaches vue: see support/renderSfc.js.
import { compileSfc, renderSfc } from './support/renderSfc.js'

const PICKER = new URL('../src/components/MemberPicker.vue', import.meta.url).pathname
const DOCUMENTS = new URL('../src/views/Documents.vue', import.meta.url).pathname

const MemberPicker = await compileSfc(PICKER)

const members = [{ email: 'ann@example.com', name: 'Ann Author' }]
const selected = { modelValue: 'ann@example.com', members }

// The closed picker has exactly one button: the clear X on the selected row.
test('default picker shows the selected member and a clear button', async () => {
  const html = await renderSfc(MemberPicker, selected)
  assert.ok(html.includes('Ann Author'), `name missing: ${html}`)
  assert.ok(html.includes('<button'), `clear button missing: ${html}`)
})

test('clearable=false keeps the selected member but renders no clear button', async () => {
  const html = await renderSfc(MemberPicker, { ...selected, clearable: false })
  assert.ok(html.includes('Ann Author'), `name missing: ${html}`)
  assert.ok(!html.includes('<button'), `clear button rendered: ${html}`)
})

test('both document Author pickers in Documents.vue are not clearable', () => {
  const src = readFileSync(DOCUMENTS, 'utf8')
  const tags = src.match(/<MemberPicker\b[^>]*?\/?>/gs) ?? []
  const authors = tags.filter(t => t.includes('author_placeholder'))
  assert.equal(authors.length, 2, `expected 2 Author pickers, found ${authors.length}`)
  for (const tag of authors) {
    assert.match(tag, /:clearable="false"/, `Author picker still clearable: ${tag}`)
  }
})
