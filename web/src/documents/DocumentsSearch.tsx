import type { RefObject } from "react";
import { Option, Select } from "../ui/Select";
import { DocumentsSearchBox } from "./DocumentsSearchBox";
import css from "./DocumentsSearch.module.css";

type DocumentsSearchProps = {
  inputRef: RefObject<HTMLInputElement | null>;
  q: string;
  setQ: (q: string) => void;
  onKey: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  kinds: { name: string; title: string }[]; // the spaces' kinds, each once
  kind: string; // "" for any
  setKind: (k: string) => void;
  states: string[]; // where one of the kind chosen can stand
  state: string; // "" for anywhere
  setState: (s: string) => void;
};

// DocumentsSearch is a search of every space's documents: what's typed,
// kept to a kind once one's chosen, and to where one stands.
export function DocumentsSearch({ inputRef, q, setQ, onKey, kinds, kind, setKind, states, state, setState }: DocumentsSearchProps) {
  return (
    <div className={css.search}>
      <DocumentsSearchBox inputRef={inputRef} q={q} setQ={setQ} placeholder="Search the documents: what they say, in a few words or a question" label="Search the documents" onKey={onKey} />
      {kinds.length > 0 && (
        <Select value={kind} onChange={(v) => (setKind(v), setState(""))} aria-label="Of a kind">
          <Option value="">Any kind</Option>
          {kinds.map((k) => (
            <Option key={k.name} value={k.name}>
              {k.title}
            </Option>
          ))}
        </Select>
      )}
      {states.length > 0 && (
        <Select value={state} onChange={setState} aria-label="Where it stands">
          <Option value="">Anywhere</Option>
          {states.map((s) => (
            <Option key={s} value={s}>
              {s}
            </Option>
          ))}
        </Select>
      )}
    </div>
  );
}
