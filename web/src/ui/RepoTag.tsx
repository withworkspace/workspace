import { chipClass } from "./Chip";
import { cx } from "./cx";
import { RepoDot } from "./RepoDot";
import css from "./RepoTag.module.css";

// RepoTag is which repo something's in, as ws labels it: its colour and
// its name, in a quiet pill.
export function RepoTag({ label, color, title, className }: { label: string; color?: string; title?: string; className?: string }) {
  return (
    <span className={chipClass("quiet", cx(css.tag, className))} title={title}>
      <RepoDot color={color} />
      {label}
    </span>
  );
}
