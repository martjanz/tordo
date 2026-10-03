// Reordering a list's rows by dragging a handle (pointer events: mouse and
// touch) or with Alt+arrows on it. Shared by a playlist and the queue.
import { dragShift, dropIndex } from './order.js'

export class RowDrag {
  drag = $state(null) // { from, to, dy, h }
  #rects = [] // the rows' boxes when the drag began
  #startY = 0

  // rows returns the row elements in order; onmove(from, to) applies a
  // move; onend runs when a drag is over (dropped or cancelled).
  constructor({ rows, onmove, onend = () => {} }) {
    this.rows = rows
    this.onmove = onmove
    this.onend = onend
  }

  down = (event, i) => {
    if (event.button > 0) return
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
    this.#rects = this.rows().map((el) => el.getBoundingClientRect())
    this.#startY = event.clientY
    this.drag = { from: i, to: i, dy: 0, h: this.#rects[i].height }
  }

  move = (event) => {
    if (!this.drag) return
    const dy = event.clientY - this.#startY
    this.drag = { ...this.drag, dy, to: dropIndex(this.#rects, this.drag.from, dy) }
  }

  up = () => {
    if (!this.drag) return
    const { from, to } = this.drag
    this.drag = null
    if (from !== to) this.onmove(from, to)
    this.onend()
  }

  cancel = () => {
    this.drag = null
    this.onend()
  }

  // key moves row i of n one place on Alt+↑/↓ and returns where it went,
  // or -1 when the key is not for it.
  key = (event, i, n) => {
    if (this.drag || !event.altKey || (event.key !== 'ArrowUp' && event.key !== 'ArrowDown')) return -1
    event.preventDefault()
    const to = i + (event.key === 'ArrowUp' ? -1 : 1)
    if (to < 0 || to >= n) return -1
    this.onmove(i, to)
    return to
  }

  // transform and transition are row i's styles while a drag goes on.
  transform = (i) => (this.drag ? `translateY(${this.drag.from === i ? this.drag.dy : dragShift(this.drag, i)}px)` : undefined)
  transition = (i) => (this.drag && this.drag.from !== i ? 'transform 0.12s' : undefined)
}
