import { useMemo, useState } from "react";
import type { ArtifactInfo, TalkDetail } from "../api";
import { RefTag } from "../refs/RefTag";
import { refKey } from "../refs/refs";
import { MoreLink } from "../ui/MoreLink";
import { made } from "./proposals";
import { TalkSide } from "./TalkSide";
import { TalkSideEmpty } from "./TalkSideEmpty";
import { TalkSideItem } from "./TalkSideItem";
import { TalkSideMade } from "./TalkSideMade";
import { TalkSideNamed } from "./TalkSideNamed";
import { TalkSideSection } from "./TalkSideSection";
import { TalkSideSummary } from "./TalkSideSummary";
import { proposalIcons } from "./words";

// SideOf is what's come of a conversation, beside it: what's waiting to
// be confirmed, what it's made (each where it is now), the capsules it
// lifted, what it names, and its summary so far. A thing to confirm goes
// to its card; a thing made goes to the thing, with its card a click
// beside it.
export function SideOf({ d, slug, id, artifacts, goTo }: { d?: TalkDetail; slug: string; id: string; artifacts?: ArtifactInfo[]; goTo: (n: number) => void }) {
  const [allNamed, setAllNamed] = useState(false);
  const items = useMemo(() => made(d?.proposals ?? [], slug, (a) => artifacts?.find((x) => x.id === a)?.title), [d?.proposals, slug, artifacts]);
  // What its turns name, once each, in the order first named: but for
  // whole repos (every turn names one) and the conversation itself.
  const named = useMemo(() => {
    const seen = new Map<string, string>();
    for (const t of d?.turns ?? [])
      for (const r of t.refs ?? []) {
        if (r.ref.kind === "repo" || (r.ref.kind === "talk" && r.ref.id === id)) continue;
        const k = refKey(r.ref);
        if (!seen.has(k)) seen.set(k, r.said);
      }
    return [...seen];
  }, [d?.turns, id]);
  const open = items.filter((m) => m.open);
  const done = items.filter((m) => !m.open);
  const shown = allNamed ? named : named.slice(0, 12);
  const icon = (kind: string) => proposalIcons[kind] ?? "pencil";
  return (
    <TalkSide>
      {open.length > 0 && (
        <TalkSideSection title="To confirm">
          <ul>
            {open.map((m) => (
              <li key={m.n}>
                <TalkSideItem open icon={icon(m.kind)} name={m.name} tag={m.tag} onClick={() => goTo(m.n)} />
              </li>
            ))}
          </ul>
        </TalkSideSection>
      )}
      {done.length > 0 && (
        <TalkSideSection title="Made here">
          <ul>
            {done.map((m) => (
              <TalkSideMade key={m.n} onWhere={m.to ? () => goTo(m.n) : undefined}>
                <TalkSideItem grow icon={icon(m.kind)} name={m.name} tag={m.tag} to={m.to} onClick={() => goTo(m.n)} />
              </TalkSideMade>
            ))}
          </ul>
        </TalkSideSection>
      )}
      {(d?.capsules?.length ?? 0) > 0 && (
        <TalkSideSection title="Capsules" tip="Capsules its agent lifted to try things in: the conversation's own, not a mission's, and put away with it">
          <TalkSideNamed>
            {d!.capsules!.map((c) => (
              <li key={`${c.repo}/${c.name}`} title={c.away ? `Put away with the conversation, its branch (${c.branch}) kept${c.kept ? ", with what wasn't committed" : ""}` : `On branch ${c.branch}`}>
                <code>
                  {c.repo}/{c.name}
                </code>
                {c.away && <span className="dim"> put away</span>}
              </li>
            ))}
          </TalkSideNamed>
        </TalkSideSection>
      )}
      {named.length > 0 && (
        <TalkSideSection title="Mentioned">
          <TalkSideNamed refs>
            {shown.map(([k, said]) => (
              <li key={k}>
                <RefTag refKey={k}>{said}</RefTag>
              </li>
            ))}
          </TalkSideNamed>
          {named.length > shown.length && (
            <MoreLink onClick={() => setAllNamed(true)}>
              {named.length - shown.length} more
            </MoreLink>
          )}
        </TalkSideSection>
      )}
      {d?.summary && (
        <TalkSideSection title="Summary so far">
          <TalkSideSummary text={d.summary} />
        </TalkSideSection>
      )}
      {d && !items.length && !named.length && !d.summary && !d.capsules?.length && <TalkSideEmpty />}
    </TalkSide>
  );
}
