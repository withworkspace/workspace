import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useEffect, useRef, useState } from "react";
import type { ThreadKind } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { NoteBody, NoteCard, NoteHead, NoteHint, noteHintActionClass, NoteId, NoteThread, NoteTurn } from "../ui/NoteCard";
import { Spinner } from "../ui/Spinner";
import { AnnotateActions } from "./AnnotateActions";
import { AnnotateAddComment } from "./AnnotateAddComment";
import { AnnotateBar, AnnotateBarNote } from "./AnnotateBar";
import { AnnotateBlockMark } from "./AnnotateBlockMark";
import { AnnotateChangeBar } from "./AnnotateChangeBar";
import { AnnotateChangePop } from "./AnnotateChangePop";
import { AnnotateComposer } from "./AnnotateComposer";
import { AnnotateCursor } from "./AnnotateCursor";
import { AnnotateDecision } from "./AnnotateDecision";
import { AnnotateEdit } from "./AnnotateEdit";
import { AnnotateKindChip } from "./AnnotateKindChip";
import { AnnotateDocCol, AnnotateLayout, annotateLandedClass, AnnotateMargin, AnnotateSlot, AnnotateText } from "./AnnotateLayout";
import { AnnotateLost } from "./AnnotateLost";
import { AnnotateLostQuote } from "./AnnotateLostQuote";
import { AnnotateMarginHint } from "./AnnotateMarginHint";
import { AnnotatePreviewBar } from "./AnnotatePreviewBar";
import { AnnotateSettled } from "./AnnotateSettled";
import { AnnotateSheet } from "./AnnotateSheet";
import { AnnotateSuggestion, AnnotateSuggestionBusy } from "./AnnotateSuggestion";
import { AnnotateThreadMarker } from "./AnnotateThreadMarker";
import { AnnotateTyping } from "./AnnotateTyping";

// A document being commented on, from its parts: the bar over it, the
// column with what's drawn over the document (the cursor, a commented
// diagram's box, what's changed since you commented and what it said),
// the margin of threads beside it, and each thing a thread holds: its
// kind, a suggested edit, a decision to record, a new comment being
// written; and on a narrow window, the markers and the sheet.

const meta: Meta = { title: "Annotate/Annotate" };
export default meta;

const none = () => {};
const pageStyle = { padding: "24px 32px", maxWidth: 1120 } as const;
const col = { display: "grid", gridTemplateColumns: "minmax(0, 1fr)", gap: 14, width: 320 } as const;
const row = { display: "flex", flexWrap: "wrap", gap: 24, alignItems: "flex-start" } as const;
const label = { marginBottom: 4 } as const;

function Labelled({ said, children }: { said: string; children: ReactNode }) {
  return (
    <div>
      <div className="dim" style={label}>
        {said}
      </div>
      {children}
    </div>
  );
}

const doc = `# Money is whole minor units

Every amount the API takes or gives is a whole number of the currency's minor unit: pence, cents. A float never holds money.

## Why

Rounding drifted a penny on invoices with many lines, and the totals on the PDF didn't match the ledger.

## What it changes

- \`amount\` is an integer everywhere.
- Formatting to pounds happens at the edge, once.`;

// Thread is one of a document's threads in the margin, as ThreadCard draws
// it, in each state.
function Thread({ kind, state, children }: { kind: ThreadKind; state?: "active" | "hovered" | "yours" | "folded"; children?: ReactNode }) {
  const folded = state === "folded";
  return (
    <NoteCard dense cursor={state === "active"} hovered={state === "hovered"} yours={state === "yours"} folded={folded} resolved={folded}>
      <NoteHead
        end={
          <>
            <NoteId>t{kind === "ask" ? 1 : kind === "change" ? 2 : 3}</NoteId>
            <Button variant="ghost" size="small">
              {folded ? "Reopen" : "Resolve"}
            </Button>
          </>
        }
      >
        <AnnotateKindChip kind={kind} />
        {folded ? (
          <Chip tone="done">
            <Icon name="check" size={11} /> Resolved
          </Chip>
        ) : state === "yours" ? (
          <Chip tone="attention">Your turn</Chip>
        ) : null}
      </NoteHead>
      {folded ? (
        <NoteBody oneLine className="dim">
          Should this say what happens to amounts already stored as floats?
        </NoteBody>
      ) : (
        children
      )}
    </NoteCard>
  );
}

// The bar over the document, as it stands: threads open and waiting, the
// document approved, and one approved at an earlier version.
export const Bar: StoryObj = {
  render: () => (
    <div style={{ ...pageStyle, display: "grid", gap: 8 }}>
      <AnnotateBar
        counts={
          <>
            <Chip tone="quiet">
              <Icon name="message" size={11} /> 3 open
            </Chip>
            <Chip tone="attention">
              <Icon name="reply" size={11} /> 1 your turn
            </Chip>
            <Chip tone="progress">
              <Spinner size={11} /> 1 answering
            </Chip>
            <Chip tone="attention">
              <Icon name="gavel" size={11} /> 1 decision to record
            </Chip>
          </>
        }
      >
        <Button>Send 2 changes to crew</Button>
        <Button variant="ghost" size="small">
          <Icon name="eye" size={13} /> Hide changes <Keys k="i" />
        </Button>
        <Button>
          <Icon name="check" size={13} /> Approve <Keys k="⇧A" />
        </Button>
      </AnnotateBar>
      <AnnotateBar>
        <Chip tone="done">
          <Icon name="check-circle" size={12} /> Approved
        </Chip>
      </AnnotateBar>
      <AnnotateBar
        counts={
          <Chip tone="quiet">
            <Icon name="message" size={11} /> 1 open
          </Chip>
        }
      >
        <AnnotateBarNote>You approved an earlier version</AnnotateBarNote>
        <Button variant="primary">
          <Icon name="check" size={13} /> Approve again <Keys k="⇧A" />
        </Button>
      </AnnotateBar>
      <AnnotatePreviewBar count={2} marked="t2" />
      <AnnotatePreviewBar count={1} marked="t4" />
    </div>
  ),
};

function Highlighted({ children }: { children: ReactNode }) {
  const at = useRef<HTMLElement>(null);
  useEffect(() => {
    // The page highlights a thread's words by the CSS Custom Highlight
    // API: here, the first words of two paragraphs, an ask in focus.
    // The markdown is drawn once it's loaded: look until it is.
    const t = setInterval(() => {
      const ps = at.current?.querySelectorAll("p");
      if (!ps || ps.length < 2 || !("highlights" in CSS)) return;
      clearInterval(t);
      const range = (p: Element, n: number) => {
        const r = new Range();
        r.setStart(p.firstChild!, 0);
        r.setEnd(p.firstChild!, n);
        return r;
      };
      const ask = range(ps[0], 40);
      const change = range(ps[1], 22);
      CSS.highlights.set("ws-note-ask", new Highlight(ask));
      CSS.highlights.set("ws-note-active", new Highlight(ask));
      CSS.highlights.set("ws-note-change", new Highlight(change));
      CSS.highlights.set("ws-note-suggest", new Highlight(change));
    }, 100);
    return () => {
      clearInterval(t);
      for (const n of ["ws-note-ask", "ws-note-active", "ws-note-change", "ws-note-suggest"]) CSS.highlights.delete(n);
    };
  }, []);
  return <AnnotateText ref={at}>{children}</AnnotateText>;
}

// The document and its margin: the cursor beside a paragraph, a
// commented block's box (on), what's changed since you commented (one
// open on what it said), the comment button beside a selection, and two
// threads in the margin, placed.
export const Document: StoryObj = {
  render: () => (
    <div style={pageStyle}>
      <AnnotateLayout>
        <AnnotateDocCol>
          <Highlighted>
            <Markdown text={doc} />
          </Highlighted>
          <AnnotateCursor top={56} height={52} />
          <AnnotateBlockMark top={300} left={-4} width={340} height={56} on />
          <AnnotateChangeBar top={130} height={48} on onPress={none} />
          <AnnotateChangeBar top={250} height={4} removed onPress={none} />
          <AnnotateChangePop top={184} old={["Rounding drifted on invoices with many lines."]} added onClose={none} />
        </AnnotateDocCol>
        <AnnotateMargin height={520}>
          <AnnotateAddComment top={0} text onPress={none} />
          <AnnotateSlot top={56}>
            <Thread kind="ask" state="active">
              <NoteBody>Does this hold for currencies with no minor unit, like the yen?</NoteBody>
              <NoteThread>
                <NoteTurn who="Agent" said="Yes: the yen's minor unit is the yen itself, so amounts are whole yen." />
              </NoteThread>
              <NoteHint>
                <Button variant="link" className={noteHintActionClass()}>
                  <Icon name="reply" size={12} /> Reply <Keys k="⏎" />
                </Button>
              </NoteHint>
            </Thread>
          </AnnotateSlot>
          <AnnotateSlot top={260}>
            <Thread kind="note" state="folded" />
          </AnnotateSlot>
          <AnnotateSlot>
            <Thread kind="change">Placed later: see-through until then.</Thread>
          </AnnotateSlot>
        </AnnotateMargin>
      </AnnotateLayout>
    </div>
  ),
};

// The margin with nothing in it, the comment button beside the cursor's
// paragraph, a removed passage's pop, and a passage as an accepted edit
// lands in it.
export const Quiet: StoryObj = {
  render: () => (
    <div style={pageStyle}>
      <AnnotateLayout>
        <AnnotateDocCol>
          <AnnotateText>
            <div>
              <p className={annotateLandedClass}>Formatting to pounds happens at the edge, once.</p>
              <p>A float never holds money.</p>
            </div>
          </AnnotateText>
          <AnnotateChangePop top={90} old={[]} added={false} onClose={none} />
        </AnnotateDocCol>
        <AnnotateMargin height={160}>
          <AnnotateAddComment top={60} onPress={none} />
          <AnnotateMarginHint />
        </AnnotateMargin>
      </AnnotateLayout>
    </div>
  ),
};

// A thread in the margin in each state, with what it can hold: an agent
// answering, what's come of it, a suggested edit, a decision drafted.
export const Threads: StoryObj = {
  render: () => (
    <div style={{ ...pageStyle, ...row }}>
      <div style={col}>
        <Labelled said="At rest, an agent answering">
          <Thread kind="ask">
            <NoteBody>Does this hold for the yen?</NoteBody>
            <AnnotateTyping>Answering</AnnotateTyping>
          </Thread>
        </Labelled>
        <Labelled said="Pointed at, waiting on you">
          <Thread kind="note" state="yours">
            <NoteBody>The ledger's export is the other place this bites.</NoteBody>
            <NoteThread>
              <NoteTurn who="Agent" said="It reads amounts as integers already.">
                <span className="dim">changed it</span>
                <span className="mono dim">3f9c2ab</span>
              </NoteTurn>
            </NoteThread>
          </Thread>
        </Labelled>
        <Labelled said="Folded, resolved">
          <Thread kind="ask" state="folded" />
        </Labelled>
      </div>
      <div style={col}>
        <Labelled said="A change, its edit suggested (in focus)">
          <Thread kind="change" state="active">
            <NoteBody>Say it once, plainly.</NoteBody>
            <AnnotateSettled>Edit declined</AnnotateSettled>
            <AnnotateEdit by="agent" from="Rounding drifted a penny on invoices with many lines." to="Totals drifted by a penny on long invoices." active onSettle={none} onPreview={none} />
          </Thread>
        </Labelled>
        <Labelled said="Its edit landing on a branch, and being written in">
          <Thread kind="change">
            <AnnotateEdit by="you" from="A float never holds money." to="" landing="main" onSettle={none} onPreview={none} />
            <AnnotateEdit by="agent" from="once" to="once, in the view" settling onSettle={none} onPreview={none} />
          </Thread>
        </Labelled>
      </div>
      <div style={col}>
        <Labelled said="Resolved, its decision drafted">
          <Thread kind="ask" state="active">
            <NoteBody>Do we keep floats in the old export?</NoteBody>
            <AnnotateDecision draftedBy="agent" what="The old export keeps floats until v3." why="Partners parse it; changing it breaks them." onWhat={none} onWhy={none} onRecord={none} onSkip={none} />
          </Thread>
        </Labelled>
        <Labelled said="In your own words, and recorded">
          <Thread kind="note" state="active">
            <AnnotateDecision what="" why="" onWhat={none} onWhy={none} onRecord={none} onCancel={none} />
            <AnnotateSettled>Recorded as d4</AnnotateSettled>
          </Thread>
        </Labelled>
        <Labelled said="Not in this version">
          <AnnotateLost said="A thread's text isn't in this version: it's been changed, or removed.">
            <Thread kind="change">
              <AnnotateLostQuote title="opened on version 2 of the document, line 7">Rounding drifted a penny on invoices with many lines.</AnnotateLostQuote>
              <NoteBody>Say how much it drifted.</NoteBody>
            </Thread>
          </AnnotateLost>
        </Labelled>
      </div>
    </div>
  ),
};

// What's in a thread, alone: each kind's chip, the box of something to
// settle with its busy line, and a line of what's pressed.
export const Pieces: StoryObj = {
  render: () => (
    <div style={{ ...pageStyle, ...row }}>
      <div style={{ display: "flex", gap: 6 }}>
        <AnnotateKindChip kind="ask" />
        <AnnotateKindChip kind="change" />
        <AnnotateKindChip kind="note" />
      </div>
      <div style={col}>
        <AnnotateSuggestion kind="edit" icon="pencil" title="Suggested edit" by="by the agent" actions={<AnnotateSuggestionBusy>Writing it in…</AnnotateSuggestionBusy>}>
          <div style={{ padding: "0 8px" }}>What's suggested</div>
        </AnnotateSuggestion>
        <AnnotateActions start={<span className="dim">At the start</span>}>
          <Button variant="ghost" size="small">
            Cancel
          </Button>
          <Button variant="primary" size="small">
            Do it
          </Button>
        </AnnotateActions>
        <AnnotateTyping>Drafting what was decided</AnnotateTyping>
      </div>
    </div>
  ),
};

function Writing({ quote, suggesting }: { quote?: string; suggesting?: boolean }) {
  const [kind, setKind] = useState<ThreadKind>(suggesting ? "change" : "ask");
  const [text, setText] = useState(suggesting ? "" : "What happens to amounts already stored?");
  const [edit, setEdit] = useState<string | undefined>(suggesting ? quote : undefined);
  return (
    <AnnotateComposer
      kind={kind}
      onKind={setKind}
      quote={quote}
      text={text}
      onText={setText}
      edit={edit}
      onEdit={setEdit}
      error={suggesting ? "The document changed under you: reload it." : undefined}
      canSuggest={!!quote}
      onSuggest={() => setEdit(quote)}
      canPost={edit !== undefined ? edit !== quote : !!text.trim()}
      onPost={none}
      postKeys="⌘⏎"
      onCancel={none}
    />
  );
}

// A new comment being written: on words (it can be an edit instead), on a
// diagram, and as an edit suggested, refused.
export const Composer: StoryObj = {
  render: () => (
    <div style={{ ...pageStyle, ...row }}>
      <div style={col}>
        <Writing quote="Every amount the API takes or gives is a whole number of the currency's minor unit" />
      </div>
      <div style={col}>
        <Writing />
      </div>
      <div style={col}>
        <Writing quote="A float never holds money." suggesting />
      </div>
    </div>
  ),
};

// On a narrow window: markers beside the text for each thread (by kind,
// waiting on you, resolved, in focus), and a thread opened in a sheet.
function NarrowDoc() {
  const [open, setOpen] = useState(true);
  return (
    <div style={{ ...pageStyle, maxWidth: 390 }}>
      <AnnotateDocCol>
        <AnnotateText>
          <Markdown text={doc} />
        </AnnotateText>
        <AnnotateThreadMarker kind="ask" top={60} turns={2} label="Ask thread t1" onPress={() => setOpen(true)} />
        <AnnotateThreadMarker kind="change" top={84} turns={1} on label="Change thread t2" onPress={() => setOpen(true)} />
        <AnnotateThreadMarker kind="note" top={150} turns={3} yours label="Note thread t3" onPress={() => setOpen(true)} />
        <AnnotateThreadMarker kind="ask" top={220} turns={1} resolved label="Ask thread t4, resolved" onPress={() => setOpen(true)} />
      </AnnotateDocCol>
      {open && (
        <AnnotateSheet label="Thread" onClose={() => setOpen(false)}>
          <Thread kind="change" state="active">
            <NoteBody>Say it once, plainly.</NoteBody>
          </Thread>
        </AnnotateSheet>
      )}
    </div>
  );
}

export const Narrow: StoryObj = { render: () => <NarrowDoc /> };
