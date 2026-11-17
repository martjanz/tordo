// Shelves: the rows of covers that scroll sideways (Inicio), and the pages
// wide enough to hold them.

// Leftover pixels a zoomed or snapped scroll may leave at an edge.
const SLACK = 1

// edges says whether a row has more to see before its start and past its
// end, from the row's scroll position. A right-to-left row scrolls into
// negative values; the distance from the start is the same.
export function edges({ scrollLeft, clientWidth, scrollWidth }) {
  const from = Math.abs(scrollLeft)
  return {
    start: from > SLACK,
    end: from + clientWidth < scrollWidth - SLACK,
  }
}

// pageStep is how far an arrow moves a row of clientWidth pixels: the
// whole cells in view but one, so the last one seen stays as a landmark.
// cell is the width of a cell plus the gap (0 if not known yet).
export function pageStep(clientWidth, cell) {
  if (!(cell > 0)) return Math.round(clientWidth * 0.8)
  return Math.max(1, Math.floor(clientWidth / cell) - 1) * cell
}

// The pages made of covers: they use the whole screen. The rest (track
// lists, settings, help) keep a width that reads well.
const WIDE = new Set(['inicio', 'explorar'])

export const widePage = (page) => WIDE.has(page)
