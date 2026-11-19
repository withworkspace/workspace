const steps = ['Add your studio', 'Write an invoice', 'Send it']

export function Checklist({ done }: { done: number }) {
  return (
    <ol className="checklist">
      {steps.map((s, i) => (
        <li key={s} aria-checked={i < done}>
          {s}
        </li>
      ))}
    </ol>
  )
}
