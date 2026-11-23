import { cx } from "../../ui/cx";
import css from "./FindScopes.module.css";

// FindScopes is what Find looks through, as a row of tabs under the text
// box: each with the letter that says it when typed first (prefix), the
// one it's looking through now (value) marked. With none marked, it's
// narrowed to something that has no tab.
export function FindScopes<T extends string>({ scopes, value, onPick }: { scopes: { id: T; label: string; prefix?: string }[]; value?: T; onPick: (id: T) => void }) {
  return (
    <div className={css.scopes} role="tablist" aria-label="Look through">
      {scopes.map((s) => (
        // The focus stays in the text box: what's typed next is asked of it.
        <button key={s.id} role="tab" aria-selected={s.id === value} className={cx(css.scope, s.id === value && css.on)} onMouseDown={(e) => e.preventDefault()} onClick={() => onPick(s.id)}>
          {s.label}
          {s.prefix && <kbd className={css.key}>{s.prefix}</kbd>}
        </button>
      ))}
    </div>
  );
}
