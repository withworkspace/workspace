import { type RefObject, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router";
import { type DocHit, docAddress, searchDocuments, type Space } from "../api";
import { Icon } from "../ui/Icon";
import { ErrorCard, humanError, Keys } from "../ui/kit";
import { Option, Select } from "../ui/Select";
import { TLink } from "../ui/TLink";
import { Chip } from "../ui/Chip";
import { GroupHead } from "../ui/GroupHead";
import { Row, RowLine, RowMain, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";
import { Input } from "../ui/Input";
import { useThingMenu } from "../shell/ThingMenu";

// Search over the spaces' documents: ranked, each led to the section
// that says it best; kept to a kind, and where one stands, once a kind's
// chosen. Only the default branch is searched: a document in flight is
// read as it is there, in its space.

export const hitTo = (h: DocHit) => docAddress(`${h.repo}:${h.path}`) + (h.anchor ? `#${h.anchor}` : "");

export function SearchBox({ q, setQ, inputRef, spaces, kind, setKind, state, setState, onKey }: { q: string; setQ: (q: string) => void; inputRef: RefObject<HTMLInputElement | null>; spaces: Space[]; kind: string; setKind: (k: string) => void; state: string; setState: (s: string) => void; onKey: (e: React.KeyboardEvent) => void }) {
  // The kinds the spaces have, each once, and where one can stand.
  const kinds = useMemo(() => {
    const out = new Map<string, { title: string; states?: string[] }>();
    for (const s of spaces) for (const k of s.kinds) out.set(k.name, { title: k.title, states: k.states });
    return out;
  }, [spaces]);
  const states = kinds.get(kind)?.states ?? [];
  return (
    <div className="doc-search">
      <label className="doc-search-box">
        <Icon name="search" size={14} className="dim" />
        <Input
          ref={inputRef}
          value={q}
          placeholder="Search the documents: what they say, in a few words or a question"
          aria-label="Search the documents"
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={onKey}
        />
        {!q && <Keys k="/" />}
      </label>
      {kinds.size > 0 && (
        <Select value={kind} onChange={(v) => (setKind(v), setState(""))} aria-label="Of a kind">
          <Option value="">Any kind</Option>
          {[...kinds].map(([name, k]) => (
            <Option key={name} value={name}>
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

// useDocSearch is what's found for q, kept to kind and state, as it's
// typed (a moment after the last key).
export function useDocSearch(q: string, kind: string, state: string) {
  const [hits, setHits] = useState<DocHit[]>();
  const [more, setMore] = useState(0);
  const [error, setError] = useState<string>();
  useEffect(() => {
    if (!q.trim()) {
      setHits(undefined);
      return;
    }
    let stale = false;
    const t = setTimeout(() => {
      searchDocuments(q, { kind, state, limit: 30 }).then(
        (r) => {
          if (stale) return;
          setHits(r.hits);
          setMore(r.more);
          setError(undefined);
        },
        (e: Error) => !stale && setError(humanError(e.message)),
      );
    }, 200);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [q, kind, state]);
  return { hits, more, error };
}

export function Results({ hits, more, error, index, setIndex }: { hits?: DocHit[]; more: number; error?: string; index: number; setIndex: (i: number) => void }) {
  const navigate = useNavigate();
  const menu = useThingMenu();
  if (error) return <ErrorCard title="Couldn't search" error={error} />;
  if (!hits) return <p className="dim documents-empty">Searching…</p>;
  if (!hits.length) return <p className="dim documents-empty">No documents say that.</p>;
  return (
    <section aria-label="Found">
      <GroupHead count={hits.length + more}>
        Found
      </GroupHead>
      <Rows className="doc-hits" role="listbox" aria-label="Documents found">
        {hits.map((h, i) => (
          <Row selected={i === index} key={`${h.repo}:${h.path}`} id={`hit-${i}`} role="option" aria-selected={i === index} onMouseMove={() => i !== index && setIndex(i)}>
            <TLink to={hitTo(h)} className={rowLinkClass("hit-link")} onContextMenu={menu(hitTo(h))} onClick={(e) => (e.preventDefault(), navigate(hitTo(h)))}>
              <Icon name="book" size={16} className={rowStatusClass()} />
              <RowMain>
                <RowLine>
                  <RowTitle>
                    {h.title}
                    {h.heading && <span className="dim"> › {h.heading}</span>}
                  </RowTitle>
                  {h.state && <Chip tone="quiet">{h.state}</Chip>}
                  {!h.every && (
                    <Chip tone="quiet" title="Not every word was found in it">
                      some of the words
                    </Chip>
                  )}
                </RowLine>
                {h.snippet.length > 0 && (
                  <RowLine className="doc-snippet">
                    <span>
                      {h.snippet.map((p, j) => (p.mark ? <mark key={j}>{p.text}</mark> : <span key={j}>{p.text}</span>))}
                    </span>
                  </RowLine>
                )}
              </RowMain>
              <span className="dim doc-hit-where">{h.label}</span>
            </TLink>
          </Row>
        ))}
      </Rows>
      {more > 0 && <p className="dim documents-empty">{more} more: say more of what you're after.</p>}
    </section>
  );
}
