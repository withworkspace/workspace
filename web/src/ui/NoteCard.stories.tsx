import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "./Button";
import { Chip } from "./Chip";
import { Excerpt } from "./Excerpt";
import { Icon } from "./Icon";
import { Keys } from "./Keys";
import { NoteBody, NoteCard, NoteHead, NoteHint, noteHintActionClass, NoteId, NoteStatus, NoteThread, NoteTurn } from "./NoteCard";
import { ReplyBox } from "./ReplyBox";

// Note cards: a comment as it's drawn on a diff (open, with the keys on
// it, a draft, with what's been said since, resolved), what a pass found
// (and one that blocks), and the dense ones of a document's margin (at
// rest, pointed at, waiting on you, folded). The last is one being
// answered.

const meta: Meta = { title: "Components/NoteCard" };
export default meta;

const col = { display: "grid", gap: 14, maxWidth: 620 } as const;
const label = { marginBottom: 4 } as const;

function Toggle({ resolved }: { resolved?: boolean }) {
  return (
    <Button variant="ghost" size="small" title={`${resolved ? "Reopen" : "Resolve"} (x)`}>
      {resolved ? "Reopen" : "Resolve"}
    </Button>
  );
}

export const Comments: StoryObj = {
  render: () => (
    <div className="page" style={col}>
      <div>
        <div className="dim" style={label}>
          Open
        </div>
        <NoteCard>
          <NoteHead
            end={
              <>
                <NoteId>n4</NoteId>
                <Toggle />
              </>
            }
          >
            <span className="kind k-blocking">Blocking</span>
            <NoteStatus status="open">Open</NoteStatus>
            <span className="dim">line 42</span>
          </NoteHead>
          <NoteBody>The total's rounded per line, so an invoice of thirds comes out a penny short.</NoteBody>
        </NoteCard>
      </div>
      <div>
        <div className="dim" style={label}>
          The keys are on it
        </div>
        <NoteCard cursor>
          <NoteHead
            end={
              <>
                <NoteId href="https://github.com" title="Open it on GitHub">
                  GitHub
                </NoteId>
                <Toggle />
              </>
            }
          >
            <span className="kind k-question">Question</span>
            <NoteStatus status="addressed">Addressed</NoteStatus>
            <span className="dim">lines 12–18</span>
          </NoteHead>
          <NoteBody>Is the retry here on purpose? It hides a timeout from whoever called it.</NoteBody>
          <NoteThread>
            <NoteTurn who="api-1" said="It was: the upstream drops one request in a hundred. I've said so above it.">
              <NoteStatus status="addressed">Addressed</NoteStatus>
              <span className="mono dim">3f9c2ab</span>
            </NoteTurn>
            <NoteTurn who="You" said="Then say how many times it tries.">
              <NoteStatus status="reopened">Reopened</NoteStatus>
            </NoteTurn>
          </NoteThread>
          <NoteHint>
            <Keys k="⏎" /> reply
          </NoteHint>
        </NoteCard>
      </div>
      <div>
        <div className="dim" style={label}>
          A draft, whose lines aren't showing
        </div>
        <NoteCard draft cursor>
          <NoteHead
            end={
              <span className="dim">
                <Keys k="⏎" /> edit <Keys k="D" /> delete
              </span>
            }
          >
            <span className="kind k-nit">Nit</span>
            <NoteStatus>Draft</NoteStatus>
            <span className="dim">line 7</span>
          </NoteHead>
          <Excerpt lines={["func total(lines []Line) Money {", "\tvar sum Money"]} from={7} />
          <NoteBody>sum reads as a verb here.</NoteBody>
        </NoteCard>
      </div>
      <div>
        <div className="dim" style={label}>
          Resolved
        </div>
        <NoteCard resolved>
          <NoteHead
            end={
              <>
                <NoteId>n2</NoteId>
                <Toggle resolved />
              </>
            }
          >
            <span className="kind k-suggestion">Suggestion</span>
            <NoteStatus status="resolved">Resolved</NoteStatus>
            <span className="dim">line 3</span>
          </NoteHead>
        </NoteCard>
      </div>
    </div>
  ),
};

export const Found: StoryObj = {
  render: () => (
    <div className="page" style={col}>
      <NoteCard found>
        <NoteHead end={<Button size="small">Add to review</Button>}>
          <span className="kind k-found">naming</span>
          <span className="dim">line 19</span>
        </NoteHead>
        <NoteBody>exportCSV and ExportCsv are both here: one of them is the other's old name.</NoteBody>
      </NoteCard>
      <NoteCard found blocks cursor>
        <NoteHead end={<Button size="small">Add to review</Button>}>
          <span className="kind k-found k-blocking">correctness</span>
          <span className="dim">lines 30–34</span>
        </NoteHead>
        <NoteBody>The file's closed before the last row is written, so the export ends a row short.</NoteBody>
      </NoteCard>
    </div>
  ),
};

function Thread({ state, children }: { state: "rest" | "hovered" | "yours" | "folded" | "active"; children?: React.ReactNode }) {
  const resolved = state === "folded";
  return (
    <NoteCard dense className="k-ask" cursor={state === "active"} hovered={state === "hovered"} yours={state === "yours"} folded={state === "folded"} resolved={resolved}>
      <NoteHead
        end={
          <>
            <NoteId>t3</NoteId>
            <Toggle resolved={resolved} />
          </>
        }
      >
        <Chip className="kind-chip k-ask">
          <Icon name="message" size={11} />
          Ask
        </Chip>
        {resolved ? (
          <Chip tone="done">
            <Icon name="check" size={11} /> Resolved
          </Chip>
        ) : state === "yours" ? (
          <Chip tone="attention">Your turn</Chip>
        ) : null}
      </NoteHead>
      {state === "folded" ? (
        <NoteBody oneLine className="dim">
          Does the export keep the order the invoices were raised in, or sort them by customer?
        </NoteBody>
      ) : (
        <>
          <NoteBody>Does the export keep the order the invoices were raised in, or sort them by customer?</NoteBody>
          <NoteThread>
            <NoteTurn who="Agent" said="As they were raised: it reads them by id, which only goes up." />
            <NoteTurn who="ws" quiet said="Sent to api-1." />
          </NoteThread>
          {children}
        </>
      )}
    </NoteCard>
  );
}

export const Dense: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 14, width: 320 }}>
      <Thread state="rest" />
      <Thread state="hovered" />
      <Thread state="yours" />
      <Thread state="folded" />
      <Thread state="active">
        <NoteHint>
          <Button variant="link" className={noteHintActionClass()}>
            <Icon name="reply" size={12} /> Reply <Keys k="⏎" />
          </Button>
          <Button variant="link">Send to crew</Button>
        </NoteHint>
      </Thread>
    </div>
  ),
};

function Answering() {
  const [said, setSaid] = useState<string[]>([]);
  const [replying, setReplying] = useState(true);
  return (
    <NoteCard cursor>
      <NoteHead end={<NoteId>n4</NoteId>}>
        <span className="kind k-question">Question</span>
        <NoteStatus status="open">Open</NoteStatus>
      </NoteHead>
      <NoteBody>Is the retry here on purpose?</NoteBody>
      {said.length > 0 && (
        <NoteThread>
          {said.map((s, i) => (
            <NoteTurn key={i} who="You" said={s} />
          ))}
        </NoteThread>
      )}
      {replying ? (
        <ReplyBox onSend={(body) => (setSaid([...said, body]), setReplying(false))} onCancel={() => setReplying(false)} />
      ) : (
        <NoteHint>
          <Button variant="link" className={noteHintActionClass()} onClick={() => setReplying(true)}>
            <Icon name="reply" size={12} /> Reply
          </Button>
        </NoteHint>
      )}
    </NoteCard>
  );
}

export const Replying: StoryObj = {
  render: () => (
    <div className="page" style={col}>
      <Answering />
    </div>
  ),
};
