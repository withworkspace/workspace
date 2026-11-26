import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import css from "./RunLink.module.css";
import { textClass } from "../ui/Text";

// RunLink is where a running service answers: its address as words, which
// opens it in a tab of its own.
export function RunLink({ href, title, children }: { href: string; title?: string; children: React.ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className={cx(textClass("mono"), css.link)} title={title}>
      {children}
      <Icon name="arrow-up-right" size={11} />
    </a>
  );
}
