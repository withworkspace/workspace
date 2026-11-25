import type { RefObject } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import { Keys } from "../ui/Keys";
import css from "./DocumentsSearchBox.module.css";

type DocumentsSearchBoxProps = {
  inputRef: RefObject<HTMLInputElement | null>;
  q: string;
  setQ: (q: string) => void;
  placeholder: string;
  label: string; // what it searches, to a screen reader
  onKey: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  found?: string; // what's found, said at its end once something's typed (a space's: until then, the key)
  space?: boolean; // a space's, over its list
};

// DocumentsSearchBox is where a search of the documents is typed: what
// it finds as words, or the key that comes to it (/) while it's empty.
export function DocumentsSearchBox({ inputRef, q, setQ, placeholder, label, onKey, found, space }: DocumentsSearchBoxProps) {
  return (
    <label className={cx(css.box, space && css.space)}>
      <Icon name="search" size={14} className="dim" />
      <Input ref={inputRef} value={q} placeholder={placeholder} aria-label={label} onChange={(e) => setQ(e.target.value)} onKeyDown={onKey} />
      {found === undefined ? !q && <Keys k="/" /> : q.trim() ? <span className={cx("dim", css.count)}>{found}</span> : <Keys k="/" />}
    </label>
  );
}
