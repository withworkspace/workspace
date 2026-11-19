// Switch is something that's on or off, changed at once: a setting, not a
// box ticked before a form is sent. It says which it is to a screen
// reader (role switch), and the keys a button has work it.
export function Switch({ on, onChange, disabled, "aria-label": label }: { on: boolean; onChange: (on: boolean) => void; disabled?: boolean; "aria-label": string }) {
  return <button type="button" role="switch" className="switch" aria-checked={on} aria-label={label} disabled={disabled} onClick={() => onChange(!on)} />;
}
