import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// The Sonando view's big cover names the album, so without a picture Tapa
// sets the title on it like the album cards do, instead of a bare disc icon.

const text = readFileSync(join(import.meta.dirname, '..', 'components', 'Reproductor.svelte'), 'utf8')

describe('Sonando view', () => {
  it('names the album on the big cover', () => {
    const big = text.match(/<div class="big-pic">(<Tapa[^>]*>)/)?.[1]
    expect(big).toBeTruthy()
    expect(big).toMatch(/alt=\{entry\.album \|\| entry\.title\}/)
  })
})
