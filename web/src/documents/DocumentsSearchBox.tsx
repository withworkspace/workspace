import { documentsKeys, spaceKeys } from "./keys";
import { useSaid } from "../keymap/keymap";
import type { RefObject } from "react";
import { cx } from "../ui/cx";
import { Icon } from "../ui/Icon";
import { Input } from "../ui/Input";
import { Keys } from "../ui/Keys";
import css from "./DocumentsSearchBox.module.css";
import { Text, textClass } from "../ui/Text";

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
  const said = useSaid(space ? spaceKeys.search : documentsKeys.search);
  const key = said && <Keys k={said} />;
  return (
    <label className={cx(css.box, space && css.space)}>
      <Icon name="search" size={14} className={textClass("dim")} />
      <Input ref={inputRef} value={q} placeholder={placeholder} aria-label={label} onChange={(e) => setQ(e.target.value)} onKeyDown={onKey} />
      {found === undefined ? !q && key : q.trim() ? <Text dim className={css.count}>{found}</Text> : key}
    </label>
  );
}
