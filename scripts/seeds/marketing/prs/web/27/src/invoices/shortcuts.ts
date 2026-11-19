// Keys for the invoice list: j and k move, / goes to the filter.
export function onKey(e: KeyboardEvent, rows: number, at: number, move: (to: number) => void, focusFilter: () => void) {
  if (e.target instanceof HTMLInputElement) return
  if (e.key === 'j') move(Math.min(at + 1, rows - 1))
  else if (e.key === 'k') move(Math.max(at - 1, 0))
  else if (e.key === '/') {
    e.preventDefault()
    focusFilter()
  }
}
