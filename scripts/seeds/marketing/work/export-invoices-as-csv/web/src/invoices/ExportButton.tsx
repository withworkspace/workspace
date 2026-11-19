import { exportURL } from '../api'

// Downloads what the list is showing: the filter goes with it. With
// nothing to export it's there, and can't be pressed.
export function ExportButton({ filter, count }: { filter: string; count: number }) {
  if (count === 0) {
    return (
      <button className="button" disabled>
        Export
      </button>
    )
  }
  return (
    <a className="button" href={exportURL(filter)} download="invoices.csv">
      Export
    </a>
  )
}
