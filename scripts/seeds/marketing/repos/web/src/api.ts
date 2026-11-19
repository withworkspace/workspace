// What the api says, as it says it: amounts are whole minor units.
export interface Invoice {
  number: string
  customer: string
  currency: string
  subtotal: number
  tax: number
  total: number
}

const base = import.meta.env.VITE_API ?? 'http://localhost:8080'

export async function listInvoices(): Promise<Invoice[]> {
  const res = await fetch(`${base}/invoices`)
  if (!res.ok) throw new Error(`GET /invoices: ${res.status}`)
  const body: { invoices: Invoice[] } = await res.json()
  return body.invoices
}
