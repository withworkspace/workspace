import { useCallback, useEffect, useMemo, useState } from "react";
import { type Draft, MarkdownEditor } from "../../editor/LazyEditor";
import { TrackedDoc } from "../../editor/LazyTracked";
import { Markdown } from "../../markdown/LazyMarkdown";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { Spinner } from "../../ui/Spinner";
import { type Doc, edits, reading, type Settled } from "../briefpage";
import type { DocId, DocState, Guard } from "./docState";
import { MissionBriefDoc } from "./MissionBriefDoc";
import { MissionBriefDocNote } from "./MissionBriefDocNote";
import { MissionBriefDocWaiting } from "./MissionBriefDocWaiting";
import { unplaced as unplacedWords } from "./words";
import { Prose } from "../../ui/Prose";

// BriefDocPanel is the brief or the plan: its changes since the version
// before marked in it to accept or decline, one by one or all at once;
// then (or with none) the document itself, in the editor, to change as it
// reads (a guarded one, as it reads, until it's opened to edit by hand).
// Its toolbar is the tab's one bar's, while it's the one being edited.
export function BriefDocPanel({
  id,
  title,
  doc,
  empty,
  guard,
  readOnly,
  side,
  focus,
  onFocus,
  onState,
  onSubmit,
  toolbarIn,
  toolbarOn,
  onActive,
}: {
  id: DocId;
  title: string;
  doc: Doc;
  empty?: string;
  guard?: Guard;
  readOnly?: boolean;
  // Beside the other document.
  side?: boolean;
  focus?: string;
  onFocus: (id: string) => void;
  onState: (s: DocState) => void;
  onSubmit?: () => void;
  toolbarIn: HTMLElement | null;
  toolbarOn: boolean;
  onActive: () => void;
}) {
  const list = useMemo(() => (doc.was !== undefined && doc.was !== doc.text ? edits(doc.was, doc.text) : []), [doc.was, doc.text]);
  const [settled, setSettled] = useState<Settled>({});
  const [unplaced, setUnplaced] = useState(0);
  const [draft, setDraft] = useState<Draft>();
  // Opened to edit by hand (a guarded document's only: any other is).
  const [byHand, setByHand] = useState(!guard);
  const r = useMemo(() => reading(doc.was ?? doc.text, list, settled), [doc.was, doc.text, list, settled]);
  const showing = r.result ?? doc.text;
  const toSettle = r.pending.length > 0;
  // Nothing in it, and something to say of that: said, until it's opened
  // to write by hand.
  const blank = !showing.trim() && !!empty && !(guard && byHand);
  const settle = useCallback((cid: string, a: "accept" | "decline") => setSettled((x) => ({ ...x, [cid]: a })), []);
  const all = useCallback((a: "accept" | "decline") => setSettled((x) => ({ ...Object.fromEntries(list.map((e) => [e.id, a])), ...x })), [list]);
  useEffect(() => {
    onState({
      text: () => (draft?.dirty ? draft.text() : r.comes),
      changed: !!draft?.dirty || r.comes !== doc.text,
      pending: r.pending,
      base: r.base,
      editing: !toSettle && !blank && !readOnly && byHand,
      settle,
      all,
    });
  }, [r, draft, doc.text, onState, toSettle, blank, readOnly, byHand, settle, all]);
  const tracked = useMemo(() => r.pending.map((e) => ({ ...e, by: "agent" })), [r.pending]);
  return (
    <MissionBriefDoc
      id={id}
      title={title}
      side={side}
      toRead={
        toSettle && (
          <>
            {r.pending.length} {r.pending.length === 1 ? "change" : "changes"} to read
          </>
        )
      }
      actions={
        <>
          {toSettle && (
            <>
              <Button variant="ghost" size="small" type="button" onClick={() => all("decline")} title="Leave each as the draft before had it">
                Decline the rest
              </Button>
              <Button size="small" type="button" onClick={() => all("accept")} title="Take each as it's proposed">
                Accept the rest
              </Button>
            </>
          )}
          {guard && !toSettle && !readOnly && !byHand && (
            <>
              <Button
                variant="ghost" size="small"
                type="button"
                onClick={() => setByHand(true)}
                title="Change the plan yourself. ws holds a plan to a strict form: one it can't read is refused when you save it, saying every problem"
              >
                <Icon name="pencil" size={12} /> Edit by hand
              </Button>
              {guard.rewrite && (
                <Button
                  size="small"
                  type="button"
                  disabled={guard.busy}
                  aria-busy={guard.busy}
                  onClick={guard.rewrite}
                  title={guard.planned ? "Have its planner write the plan again, to fit the brief as it reads now (your edits to the brief go with it)" : "Have its planner write the plan, from the brief as it reads now"}
                >
                  {guard.busy ? <Spinner size={12} /> : <Icon name="sparkle" size={12} />} {guard.planned ? "Rewrite from the brief" : "Write it from the brief"}
                </Button>
              )}
            </>
          )}
        </>
      }
    >
      {unplaced > 0 && <MissionBriefDocNote>{unplacedWords(unplaced)}</MissionBriefDocNote>}
      {toSettle ? (
        <TrackedDoc
          text={r.base}
          changes={tracked}
          focus={focus}
          onFocus={onFocus}
          onSettle={settle}
          onUnplaced={(ids) => {
            setUnplaced((n) => n + ids.length);
            setSettled((x) => ({ ...x, ...Object.fromEntries(ids.map((cid) => [cid, "accept" as const])) }));
          }}
        />
      ) : blank ? (
        <MissionBriefDocWaiting>{empty}</MissionBriefDocWaiting>
      ) : readOnly || !byHand ? (
        <Prose>
          <Markdown text={showing} />
        </Prose>
      ) : (
        <div>
          {guard && (
            <MissionBriefDocNote>
              ws holds a plan to a strict form: each step starts with its kind, and what's under it is a line each. One it can't read is refused when you save it, saying every problem. <em>Discard my edits</em> puts it back.
            </MissionBriefDocNote>
          )}
          <MarkdownEditor
            key={showing}
            initial={showing}
            label={title}
            placeholder={guard ? "## Z, then ## Stops early, then ## Steps: a numbered list, each step starting with its kind" : (empty ?? "The goal, the context, what's in and out, and how you'll know it's done")}
            autoFocus={!!guard}
            onChange={setDraft}
            onSubmit={onSubmit}
            toolbarIn={toolbarIn}
            toolbarOn={toolbarOn}
            onFocus={onActive}
          />
        </div>
      )}
    </MissionBriefDoc>
  );
}
