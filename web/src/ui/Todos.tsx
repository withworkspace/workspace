import css from "./Todos.module.css";
import { cx } from "./cx";
import { Icon } from "./Icon";
import { Spinner } from "./Spinner";
import { SrOnly } from "./SrOnly";

// Todo is one thing on an agent's list: what's to do, where it stands,
// and how it reads while it's in hand ("Running the tests").
export interface Todo {
  content: string;
  status: string;
  active?: string;
}

// Todos is an agent's todo list, as it keeps it: done, in hand, to do.
// live is whether the agent's running, so what's in hand turns.
export function Todos({ todos, live, className }: { todos: Todo[]; live?: boolean; className?: string }) {
  return (
    <ul className={cx(css.items, className)}>
      {todos.map((t, i) => (
        <li key={i} className={cx(css.item, t.status === "completed" && css.done, t.status === "in_progress" && css.inHand)}>
          <span className={css.mark} aria-hidden>
            {t.status === "completed" ? <Icon name="check" size={10} /> : t.status === "in_progress" && live ? <Spinner size={10} /> : null}
          </span>
          <span>{t.status === "in_progress" && t.active ? t.active : t.content}</span>
          <SrOnly>{t.status === "completed" ? " (done)" : t.status === "in_progress" ? " (in hand)" : ""}</SrOnly>
        </li>
      ))}
    </ul>
  );
}
