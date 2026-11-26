import type { PassFinding } from "../api";
import { Icon } from "../ui/Icon";
import { Inline } from "../ui/Inline";
import { NoteBody, NoteCard, NoteHead } from "../ui/NoteCard";
import type { ReactNode } from "react";
import { cx } from "../ui/cx";
import css from "./ReviewFinding.module.css";
import { Suggestion } from "./Suggestion";
import { Text } from "../ui/Text";

// ReviewFinding is what a pass found, under the lines it's about: which
// pass (title), its kind (one that blocks says so), its lines, what it
// found and a change it suggests, with what's done with it (actions) in
// its head.
export function ReviewFinding({ id, title, finding: f, cursor, actions, onClick }: { id: string; title: string; finding: PassFinding; cursor: boolean; actions: ReactNode; onClick: () => void }) {
  const lines = f.endLine && f.line && f.endLine > f.line ? `lines ${f.line}–${f.endLine}` : `line ${f.line}`;
  return (
    <NoteCard id={id} className={cx(css.card, !cursor && css.lifted)} found blocks={f.blocks} cursor={cursor} onClick={onClick}>
      <NoteHead end={actions}>
        <span className={css.pass} title={`Found by the ${title} pass. Take into your review, or dismiss`}>
          <Icon name="sparkle" size={11} /> {title}
        </span>
        <span className={`kind k-found${f.blocks ? " k-blocking" : ""}`} title={f.blocks ? "Blocking: the pass fails while this is open" : undefined}>
          {f.kind}
        </span>
        <Text dim className={css.where}>
          {f.side === "old" ? "removed " : ""}
          {lines}
        </Text>
      </NoteHead>
      <NoteBody>
        <Inline text={f.text} />
      </NoteBody>
      {f.suggest ? <Suggestion from={f.excerpt} to={f.suggest} /> : null}
    </NoteCard>
  );
}
