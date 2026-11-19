export function EmptyState({ onNew }: { onNew: () => void }) {
  return (
    <div className="empty">
      <h2>Nothing sent yet</h2>
      <p>Your invoices will be listed here, with what's been paid.</p>
      <button onClick={onNew}>Write your first invoice</button>
    </div>
  )
}
