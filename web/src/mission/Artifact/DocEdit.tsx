import { useEffect, useRef, useState } from "react";
import { type ArtifactContent, type DocDraft, type DocSaved, dropDraft, getThreads, openThread, putDraft, readDoc, saveDoc, useEventsOf } from "../../api";
import { type Draft, MarkdownEditor, type Quote } from "../../editor/LazyEditor";
import { toast } from "../../ui/toast";
import { Skeleton } from "../../ui/Skeleton";
import { humanError } from "../../ui/strings";
import { Callout } from "../../ui/Callout";
import { Keys } from "../../ui/Keys";
import { Time } from "../../ui/Time";
import { Button } from "../../ui/Button";
import { Segment, Segmented } from "../../ui/Segmented";
import { MissionArtifactEdit } from "./MissionArtifactEdit";
import { MissionArtifactEditBar } from "./MissionArtifactEditBar";
import { Text } from "../../ui/Text";

// DocEdit edits one of a mission's documents in place, as it reads, or
// (no slug) a document outside any mission: a file in one of the
// workspace's repos, whose saving depends on where it is and how its repo
// lands (a.saves). One on the default branch of a repo with no pull
// requests is landed there by saving, so what the commit will say is
// shown, to change before it goes.
//
// What you write is kept as a draft with the mission as you go (not in the
// file, where crew's next commit would take it as theirs), so closing and
// coming back loses nothing. While you edit, the document's watched: if
// it changes (crew commit to it) before you've changed anything, you go on
// from it as it is; after, saving merges your edit with theirs. Where the
// two touch the same lines, the clash comes back marked, to settle in the
// markdown and save again.
//
// Or, in Suggest, your edit isn't made: each passage it changes becomes a
// suggestion of yours on a thread of its own, shown in the document as a
// tracked change (v), to accept or decline one by one.

// Start is what the editor starts from, and the version of the document
// that is (or was edited from). n counts restarts: each is a fresh editor.
interface Start {
  text: string;
  base: string;
  mode: "rich" | "source";
  n: number;
}

const clashMark = /^<{7} your edit$/m;

export function DocEdit({
  slug,
  a,
  draft,
  onSaved,
  onClose,
}: {
  slug: string;
  a: ArtifactContent;
  draft?: DocDraft | null;
  // onSaved: saved (or suggested); saved.ref says where it is now, when
  // saving moved it.
  onSaved: (saved?: DocSaved) => void;
  // onClose: left without saving; kept says a draft's kept to come back to.
  onClose: (kept: boolean) => void;
}) {
  const [start, setStart] = useState<Start>(() => ({ text: draft?.text ?? a.text, base: draft?.base ?? a.version, mode: "rich", n: 0 }));
  const [edit, setEdit] = useState<Draft>();
  // Landed by saving, the commit's message is the only description there'll
  // be: it starts as what ws would write, to change.
  const lands = a.saves?.how === "land";
  const [message, setMessage] = useState(lands ? (a.saves?.subject ?? "") : "");
  const [busy, setBusy] = useState(false);
  const [kept, setKept] = useState(draft?.at);
  const [clashes, setClashes] = useState(0);
  // The document's changed since the edit began: saving will merge.
  const [moved, setMoved] = useState(!!draft && draft.base !== a.version);
  const [quotes, setQuotes] = useState<Quote[] | undefined>(a.annotatable ? undefined : []);
  // Edit makes the change; Suggest offers it, on threads (a document to
  // comment on only).
  const [mode, setMode] = useState<"edit" | "suggest">("edit");

  // Unsaved: there's something of yours not in the document (an edit, a
  // draft come back to, a clash to settle).
  const unsaved = !!edit?.dirty || !!draft || start.n > 0;
  const live = useRef({ unsaved, edit, start, mode });
  live.current = { unsaved, edit, start, mode };

  // The words open threads are on, highlighted while you edit.
  useEffect(() => {
    if (!a.annotatable) return;
    let stale = false;
    getThreads(slug, a.ref).then(
      (st) =>
        !stale &&
        setQuotes(
          st.threads
            .filter((t) => t.status === "open" && t.anchor.quote?.exact)
            .map((t) => ({ exact: t.anchor.quote!.exact, prefix: t.anchor.quote!.prefix, kind: t.kind })),
        ),
      () => !stale && setQuotes([]),
    );
    return () => {
      stale = true;
    };
  }, [slug, a.ref, a.annotatable]);

  // Kept as you go, a moment after you stop typing.
  const keep = (text: string, base: string) =>
    putDraft(slug, { ref: a.ref, base, text }).then(
      (r) => setKept(r.draft.at),
      () => {},
    );
  useEffect(() => {
    if (!edit || !(edit.dirty || start.n > 0 || draft)) return;
    const t = setTimeout(() => keep(edit.text(), start.base), 700);
    return () => clearTimeout(t);
    // keep is the same each time but for its closure over a.ref and slug.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [edit, start]);

  // The document, followed while you edit (doc:<slug>/<ref>): read again
  // when its file's written, to see whether it's moved from what you're
  // editing.
  useEventsOf([`doc:${slug}/${a.ref}`], {
    changed: () =>
      readDoc(slug, a.ref).then(
        (now) => {
          const { unsaved, start } = live.current;
          if (now.version === start.base) return setMoved(false);
          if (unsaved) return setMoved(true);
          setStart((s) => ({ text: now.text, base: now.version, mode: s.mode, n: s.n }));
          toast(slug ? "Document updated by crew" : "Document updated", { tone: "info" });
        },
        () => {},
      ),
  });

  const close = () => {
    const { unsaved, edit, start } = live.current;
    if (unsaved && edit) keep(edit.text(), start.base);
    onClose(unsaved);
  };

  const discard = () => {
    dropDraft(slug, a.ref).finally(() => onClose(false));
  };

  // suggest offers the edit as suggestions, from the document as it is:
  // one changed since you started is edited (saving merges) or started
  // again, not suggested on.
  const suggest = async (text: string) => {
    setBusy(true);
    try {
      const now = await readDoc(slug, a.ref);
      if (now.version !== live.current.start.base) {
        setMoved(true);
        return toast("Changed since you started. Saving will merge.", { tone: "danger" });
      }
      const { suggestionsFor } = await import("../../editor/suggest");
      const proposed = suggestionsFor(now.text, text);
      if (!proposed.length) return toast("Nothing to suggest: no changes", { tone: "info" });
      for (const p of proposed) {
        await openThread(slug, a.ref, {
          version: now.version,
          line: p.line,
          endLine: p.endLine,
          quote: { exact: p.quote },
          kind: "change",
          body: message.trim() || "Suggested in the editor.",
          suggestion: p.to,
          from: p.from,
        });
      }
      await dropDraft(slug, a.ref);
      toast(proposed.length === 1 ? "Suggested: v shows in place" : `${proposed.length} edits suggested: v shows in place`, { tone: "done" });
      onSaved();
    } catch (e) {
      toast(humanError((e as Error).message), { tone: "danger" });
    } finally {
      setBusy(false);
    }
  };

  const save = () => {
    const { edit, start } = live.current;
    if (!edit || busy) return;
    if (!live.current.unsaved) return onClose(false);
    const text = edit.text();
    if (!text.trim()) return toast("Document cannot be empty", { tone: "danger" });
    if (clashMark.test(text)) return toast("Resolve conflicts first: remove the <<<<<<<, ======= and >>>>>>> lines", { tone: "danger" });
    if (live.current.mode === "suggest") return void suggest(text);
    setBusy(true);
    saveDoc(slug, { ref: a.ref, base: start.base, text, message: message.trim() || undefined })
      .then(
        (r) => {
          if (r.conflict) {
            setClashes(r.conflict.clashes);
            setMoved(false);
            setStart((s) => ({ text: r.conflict!.text, base: r.version, mode: "source", n: s.n + 1 }));
            keep(r.conflict.text, r.version);
            return;
          }
          toast(r.message ?? "Saved", { tone: "done", ms: r.ref ? 7000 : undefined });
          onSaved(r);
        },
        (e: Error) => toast(humanError(e.message), { tone: "danger" }),
      )
      .finally(() => setBusy(false));
  };

  // Mission files are written, not committed: nothing to say about it,
  // unless it's what suggestions' threads open with.
  const suggesting = mode === "suggest";
  const commits = suggesting || !a.ref.startsWith("mission:");
  return (
    <MissionArtifactEdit>
      {a.saves && a.saves.how !== "capsule" && !suggesting && (
        <Callout tone={lands ? "attention" : "info"} icon={lands ? "alert" : "info"} title={lands ? `Saving lands on ${a.saves.branch}` : `Saving creates a new capsule`}>
          {lands ? (
            <>
              No pull requests in this repo: commits straight to {a.saves.branch}
              {a.saves.verify ? (
                <>
                  , once <Text mono>{a.saves.verify}</Text> passes
                </>
              ) : null}
              . Visible to everyone at once. The message below is the only description.
            </>
          ) : (
            <>
              {a.saves.branch} is not changed directly: your edit is committed to a new capsule, for you to
              add to or open a pull request from.
            </>
          )}
        </Callout>
      )}
      {moved && (
        <Callout tone="attention" title="Changed since you started">
          {slug ? "Someone (likely crew) changed this document while you were editing." : "This document changed while you were editing."}{" "}
          Saving will merge; where lines conflict, you choose.
        </Callout>
      )}
      {clashes > 0 && (
        <Callout tone="danger" title={clashes === 1 ? "1 conflict" : `${clashes} conflicts`}>
          Your edit and other changes touch the same lines. Each conflict is marked below:{" "}
          <code>&lt;&lt;&lt;&lt;&lt;&lt;&lt; your edit</code>, then <code>=======</code> and the current
          document, then <code>&gt;&gt;&gt;&gt;&gt;&gt;&gt;</code>. Keep what should stay, remove the marker
          lines, and save.
        </Callout>
      )}
      {quotes === undefined ? (
        <Skeleton rows={8} />
      ) : (
        <MarkdownEditor
          key={start.n + ":" + start.base}
          initial={start.text}
          startIn={start.mode}
          quotes={quotes}
          label={`Editing ${a.label || a.ref}`}
          autoFocus
          onChange={setEdit}
          onSubmit={save}
          onCancel={close}
        />
      )}
      <MissionArtifactEditBar
        mode={
          a.annotatable && (
            <Segmented value={suggesting ? "suggest" : "edit"} onChange={setMode} aria-label="Edit or suggest">
              <Segment value="edit" title="Save changes directly">
                Edit
              </Segment>
              <Segment value="suggest" title="Offer each change as a suggestion to accept or decline">
                Suggest
              </Segment>
            </Segmented>
          )
        }
        message={
          commits
            ? {
                value: message,
                placeholder: suggesting ? "Why (shown on each suggestion)" : `What changed (default: "${a.saves?.subject ?? a.docKind?.subject ?? "Edit …"}")`,
                label: suggesting ? "Why" : "Commit message",
                onChange: setMessage,
                onKeyDown: (e) => {
                  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                    e.preventDefault();
                    save();
                  }
                  if (e.key === "Escape") {
                    e.preventDefault();
                    close();
                  }
                },
              }
            : undefined
        }
        kept={
          kept && unsaved ? (
            <>
              Draft saved <Time iso={kept} as="relative" />
            </>
          ) : null
        }
        actions={
          <>
            {unsaved && (
              <Button variant="ghost" onClick={discard} disabled={busy} title="Discard edit">
                Discard
              </Button>
            )}
            <Button variant="ghost" onClick={close} disabled={busy} title="Close and keep draft" keys="esc">
              Close <Keys k="esc" />
            </Button>
            <Button variant="primary" onClick={save} disabled={busy || !unsaved} keys="mod+⏎">
              {busy ? (suggesting ? "Suggesting…" : lands ? "Landing…" : "Saving…") : suggesting ? "Suggest" : lands ? `Save and land on ${a.saves?.branch}` : "Save"} <Keys k="mod+⏎" />
            </Button>
          </>
        }
      />
    </MissionArtifactEdit>
  );
}
