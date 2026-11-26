import { HoverCardHead } from "../../ui/HoverCard";
import { Icon, type IconName } from "../../ui/Icon";
import css from "./FindSaidCard.module.css";
import { Text, textClass } from "../../ui/Text";

// FindSaidCard says, in Find's pane, of something ws records that was
// found by what it says: its title, what's known of it (about: what it
// is, where, its state, each said after the last), and its words there,
// those that matched marked (snippet). All of it wraps to the pane.
export function FindSaidCard({ icon = "message", title, about, snippet }: { icon?: IconName; title: string; about: string[]; snippet?: { text: string; mark?: boolean }[] }) {
  return (
    <div className={css.card}>
      <HoverCardHead top>
        <Icon name={icon} size={13} className={textClass("dim")} />
        <span className={css.title}>{title}</span>
      </HoverCardHead>
      <Text dim>{about.join(" · ")}</Text>
      {snippet && snippet.length > 0 && <span className={css.said}>{snippet.map((p, j) => (p.mark ? <mark key={j}>{p.text}</mark> : <span key={j}>{p.text}</span>))}</span>}
    </div>
  );
}
