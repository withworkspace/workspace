import { useEffect, useState } from 'react'
import { listInvoices, type Invoice } from '../api'
import { ExportButton } from './ExportButton'
import { formatMoney } from './money'

export function InvoiceList() {
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [filter, setFilter] = useState('')

  useEffect(() => {
    listInvoices().then(setInvoices)
  }, [])

  const shown = invoices.filter((inv) => inv.customer.toLowerCase().includes(filter.toLowerCase()))

  return (
    <section className="invoices">
      <header className="toolbar">
        <h1>Invoices</h1>
        <input
          type="search"
          placeholder="Filter by customer"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <ExportButton filter={filter} count={shown.length} />
      </header>
      <table>
        <thead>
          <tr>
            <th>Number</th>
            <th>Customer</th>
            <th className="amount">Total</th>
          </tr>
        </thead>
        <tbody>
          {shown.map((inv) => (
            <tr key={inv.number}>
              <td>{inv.number}</td>
              <td>{inv.customer}</td>
              <td className="amount">{formatMoney(inv.total, inv.currency)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
