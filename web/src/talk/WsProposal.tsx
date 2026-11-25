import { useState } from "react";
import { type Draft, MarkdownEditor } from "../editor/LazyEditor";
import { Input } from "../ui/Input";
import { TextArea } from "../ui/TextArea";
import type { ProposalCardProps } from "./ProposalCard";
import { ProposalOutcome } from "./ProposalOutcome";
import { ProposalShown } from "./ProposalShown";
import { startsMission } from "./proposals";
import { proposalAccept, proposalBusy, proposalLabel } from "./proposalWords";
import { RevisionChanges } from "./RevisionChanges";
import { TalkProposal } from "./TalkProposal";
import { TalkProposalChoice } from "./TalkProposalChoice";
import { TalkProposalEdit } from "./TalkProposalEdit";
import { proposalIcons } from "./words";
import { Text } from "../ui/Text";

// WsProposal is a card of ws's own (ProposalCard says which there are):
// what's proposed, edited first or not (what the fields hold until it's
// confirmed), and settling it.
export function WsProposal({ p, revisedFrom, kinds, artifacts, frameOf, inReview, slug, replying, anchor, onSettle }: ProposalCardProps) {
  const [editingOpen, setEditing] = useState(false);
  // Once it's settled, it shows what was proposed, edited or not.
  const editing = editingOpen && p.state === "open";
  const [busy, setBusy] = useState(false);
  const [what, setWhat] = useState(p.what ?? "");
  const [why, setWhy] = useState(p.why ?? "");
  const [title, setTitle] = useState(p.title ?? "");
  // The brief, as the editor has it: its markdown's worked out on accept.
  const [brief, setBrief] = useState<Draft>();
  const [text, setText] = useState(p.text ?? "");
  // The plan is the brief's editor's too.
  const proposedText = p.kind === "plan" ? (p.plan ?? "") : (p.brief ?? "");
  const briefText = () => (brief ? brief.text() : proposedText);
  const briefEmpty = brief ? brief.empty : !proposedText.trim();
  // A document, as the editor has it; and its kind, as the workspace names it.
  const [doc, setDoc] = useState<Draft>();
  const docText = () => (doc ? doc.text() : (p.text ?? ""));
  const docEmpty = doc ? doc.empty : !p.text?.trim();
  const kindTitle = kinds?.find((k) => k.name === p.docKind)?.title ?? p.docKind ?? "Document";
  // An artifact: its note, as the editor has it; and the artifact a
  // version's of, as the workspace has it now.
  const [note, setNote] = useState(p.note ?? "");
  const artifact = p.to ? artifacts?.find((a) => a.id === p.to) : undefined;
  const edits = () => {
    if (!editing) return {};
    switch (p.kind) {
      case "decision":
        return { what, why };
      case "mission":
        return { title, brief: briefText() };
      case "brief":
        return { brief: briefText() };
      case "plan":
        return { plan: briefText() };
      case "dispatch":
      case "replan":
      case "comment":
        return { text };
      case "document":
        return { text: docText() };
      case "artifact":
        return { title, note };
    }
  };
  const run = (action: "accept" | "decline") => {
    if (busy) return;
    setBusy(true);
    onSettle(p.n, action, action === "accept" ? edits() : {}).finally(() => setBusy(false));
  };
  // Briefing a mission, the brief comes with its name and capsules.
  const briefing = startsMission(p);
  // A review comment: the pull request's page, at the lines it's on.
  const [prRepo, prNumber] = (p.pr ?? "").split("#");
  const prHref = p.pr ? `/reviews/${prRepo}/${prNumber}` : "";
  const lines = p.line ? (p.endLine && p.endLine > p.line ? `${p.line}–${p.endLine}` : `${p.line}`) : "";
  const ready = {
    decision: what.trim(),
    mission: title.trim(),
    brief: !briefEmpty,
    dispatch: text.trim(),
    replan: text.trim(),
    comment: text.trim() || p.suggestion != null,
    plan: !briefEmpty,
    document: !docEmpty,
    artifact: note.trim() && (p.to || title.trim()),
    plugin: true,
  }[p.kind];
  const rows = (s: string) => Math.min(10, Math.max(3, s.split("\n").length + 1));
  const editor = () => {
    switch (p.kind) {
      case "decision":
        return (
          <TalkProposalEdit>
            <TextArea value={what} rows={2} onChange={(e) => setWhat(e.target.value)} placeholder="The decision, in a sentence" aria-label="The decision" />
            <TextArea value={why} rows={2} onChange={(e) => setWhy(e.target.value)} placeholder="Why" aria-label="Why" />
          </TalkProposalEdit>
        );
      case "mission":
        return (
          <TalkProposalEdit>
            <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Title" aria-label="Title" />
            <MarkdownEditor initial={p.brief ?? ""} label="The brief" placeholder="The brief (none: an idea, to brief later)" compact onChange={setBrief} onCancel={() => setEditing(false)} />
          </TalkProposalEdit>
        );
      case "brief":
        return (
          <TalkProposalEdit>
            <MarkdownEditor initial={p.brief ?? ""} label="The brief" compact autoFocus onChange={setBrief} onCancel={() => setEditing(false)} />
          </TalkProposalEdit>
        );
      case "plan":
        return (
          <TalkProposalEdit>
            <MarkdownEditor initial={p.plan ?? ""} label="The plan" compact autoFocus onChange={setBrief} onCancel={() => setEditing(false)} />
          </TalkProposalEdit>
        );
      case "dispatch":
        return (
          <TalkProposalEdit>
            <TextArea value={text} rows={rows(text)} onChange={(e) => setText(e.target.value)} placeholder="What the crew's to do" aria-label="What the crew's to do" />
          </TalkProposalEdit>
        );
      case "replan":
        return (
          <TalkProposalEdit>
            <TextArea value={text} rows={rows(text)} onChange={(e) => setText(e.target.value)} placeholder="What to change in the plan" aria-label="What to change in the plan" />
          </TalkProposalEdit>
        );
      case "comment":
        return (
          <TalkProposalEdit>
            <TextArea value={text} rows={rows(text)} onChange={(e) => setText(e.target.value)} placeholder="What the comment says" aria-label="What the comment says" />
          </TalkProposalEdit>
        );
      case "document":
        return (
          <TalkProposalEdit>
            <MarkdownEditor initial={p.text ?? ""} label="The document" compact autoFocus onChange={setDoc} onCancel={() => setEditing(false)} />
            <Text as="div" dim>
              {p.to
                ? "The whole document, as it's saved: it stays where it is, whatever its first heading says."
                : "Its first heading is its title: where it goes, and its commit's message, are made from it when it's added."}
            </Text>
          </TalkProposalEdit>
        );
      case "artifact":
        return (
          <TalkProposalEdit>
            {!p.to && <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Its title" aria-label="Its title" />}
            <TextArea
              value={note}
              rows={Math.min(6, Math.max(2, note.split("\n").length + 1))}
              onChange={(e) => setNote(e.target.value)}
              placeholder={p.to ? "What changed, and why" : "What it tries out"}
              aria-label="Its note"
            />
            <Text as="div" dim>The HTML is the agent's: ask it for another version to change that.</Text>
          </TalkProposalEdit>
        );
    }
  };
  // Its title: a mission's, a document's or a brief's that starts one,
  // or an artifact's (the artifact's own, for a version of it).
  const shownTitle = editing ? undefined : p.kind === "mission" || p.kind === "document" || briefing ? p.title || undefined : p.kind === "artifact" ? p.title || artifact?.title : undefined;
  return (
    <TalkProposal
      state={p.state}
      anchor={anchor}
      icon={proposalIcons[p.kind] ?? "pencil"}
      label={proposalLabel(p, briefing, kindTitle)}
      title={shownTitle}
      revises={p.revises}
      onEdit={p.state === "open" && !editing ? () => setEditing(true) : undefined}
      changes={revisedFrom && p.state === "open" ? <RevisionChanges was={revisedFrom} now={p} /> : undefined}
      foot={
        p.state === "open" ? (
          <TalkProposalChoice
            busy={busy ? proposalBusy(p) : undefined}
            accept={proposalAccept(p, briefing, kindTitle, !!(editing ? !briefEmpty : p.brief), artifact)}
            ready={!!ready}
            replying={replying}
            onAccept={() => run("accept")}
            onDecline={() => run("decline")}
          />
        ) : (
          <ProposalOutcome p={p} briefing={briefing} slug={slug} prHref={prHref} inReview={inReview} />
        )
      }
    >
      {editing ? editor() : <ProposalShown p={p} briefing={briefing} prHref={prHref} lines={lines} frame={frameOf?.(p.n) ?? ""} artifact={artifact} />}
    </TalkProposal>
  );
}
