import { type ReactNode, useEffect, useState } from "react";
import css from "./Copyable.module.css";
import { cx } from "./cx";
import { Icon } from "./Icon";
import { clicked, keyHint, titleWithKeys } from "./keyHint";
import { toast } from "./toast";

// Copyable is a ref, path, hash or command that copies itself when
// clicked, and says so. Its words are the page's to style, by the class
// it passes. Its icon shows after the words as they're pointed at;
// icon="over" has it over their end, where the words fill the room, and
// icon="inline" has it always there, in the line.
export function Copyable({
  text,
  children,
  className,
  title,
  keys,
  icon = "after",
}: {
  text: string;
  children?: ReactNode;
  className?: string;
  title?: string;
  keys?: string; // the key that copies it, said after its title and hinted at (ui/keyHint.ts)
  icon?: "after" | "over" | "inline";
}) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 1200);
    return () => clearTimeout(t);
  }, [copied]);
  return (
    <button
      type="button"
      // The library's, so not styles.css's to draw as a button.
      data-ui=""
      className={cx(css.copyable, copied && css.copied, icon === "over" && css.over, icon === "inline" && css.inline, className)}
      title={copied ? "Copied" : titleWithKeys(title ?? `Copy ${text}`, keys)}
      onClick={(e) => {
        if (keys && clicked(e)) keyHint(keys);
        e.preventDefault();
        e.stopPropagation();
        navigator.clipboard?.writeText(text).then(() => setCopied(true), () => toast("Could not copy: browser refused"));
      }}
    >
      <span className={css.text}>{children ?? text}</span>
      <Icon name={copied ? "check" : "copy"} size={12} className={css.icon} />
    </button>
  );
}
