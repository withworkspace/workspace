import type { PassFinding } from "../api";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { NoteBody, NoteCard, NoteHead } from "../ui/NoteCard";
import type { ReactNode } from "react";
import css from "./ReviewFinding.module.css";
import { Suggestion } from "./Suggestion";

// ReviewFinding is what a pass found, under the lines it's about: which
// pass (title), its kind (one that blocks says so), its lines, what it
// found and a change it suggests, with what's done with it (actions) in
// its head.
export function ReviewFinding({ id, title, finding: f, cursor, actions, onClick }: { id: string; title: string; finding: PassFinding; cursor: boolean; actions: ReactNode; onClick: () => void }) {
  const lines = f.endLine && f.line && f.endLine > f.line ? `lines ${f.line}–${f.endLine}` : `line ${f.line}`;
  return (
    <NoteCard id={id} className={css.card} found blocks={f.blocks} cursor={cursor} onClick={onClick}>
      <NoteHead end={actions}>
        <span className={css.pass} title={`Found by the ${title} pass. Nobody's said it: take it into your review, or dismiss it`}>
          <Icon name="sparkle" size={11} /> {title}
        </span>
        <span className={`kind k-found${f.blocks ? " k-blocking" : ""}`} title={f.blocks ? "A finding of this kind blocks: the pass is no go while it's open" : undefined}>
          {f.kind}
        </span>
        <span className={`dim ${css.where}`}>
          {f.side === "old" ? "removed " : ""}
          {lines}
        </span>
      </NoteHead>
      <NoteBody>
        <Inline text={f.text} />
      </NoteBody>
      {f.suggest ? <Suggestion from={f.excerpt} to={f.suggest} /> : null}
    </NoteCard>
  );
}
