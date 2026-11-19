import type { Invoice } from '../api'

// Newest number first: INV-0042 before INV-0041.
export function byNumber(a: Invoice, b: Invoice): number {
  return b.number.localeCompare(a.number, undefined, { numeric: true })
}
