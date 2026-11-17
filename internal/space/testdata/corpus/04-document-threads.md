# Document threads

Status: built on `v2`. Live threads (phase 1), the thread agent answering (phase 2, Claude Code), suggested edits accepted and committed or declined (phase 3), and the document (phase 4: approving it and accepting its stage from there, Change threads holding up the stage, sending threads to crew, and drafting a decision on resolving) are built. A suggestion can be seen applied: `v` (or *See it applied* on its card) shows the document with it, the passage it changes marked. Part of [context and conversation](README.md).

## Why

Reviewing a document was built as code review: write comments, submit them together with a verdict, crew answer later, you re-review. That suits code, where a review is a judgement of a change. A proposal is different: you read it, ask about a sentence, want a paragraph changed, talk it through. That's how comments work in a shared doc, and it's what this does.

- **A comment is a thread,** live the moment you post it. No draft, no submitting.
- **Every thread gets an answer, in the thread.** An agent answers a question, or suggests the change you asked for.
- **A suggestion is a tracked change.** Accept it and it's written into the document.
- **You resolve threads.** The document is approved, and its stage accepted, when you're happy with it.

## Threads

A thread is a [conversation](03-conversations.md) anchored to text in a document: the quote, the markdown lines it's in, and the document's version, as built for reviewing documents. Anchoring, re-finding text after edits, highlights, and listing threads whose text has gone are unchanged.

| Kind | What you want | What the agent does |
|---|---|---|
| **Ask** | An answer. | Answers in the thread. |
| **Change** | The document changed. | Suggests an edit, with a line on why it's right; or, if the change is bigger than an edit, says so and offers to send it to crew. |
| **Note** | To say something. | Nothing, unless you ask it something later in the thread. |

Blocking, suggestion, question and nit stay with code review.

A thread is **open** until you **resolve** it, and can be reopened. While it's open it's waiting on someone:

- **the agent:** it's answering;
- **you:** it answered, or suggested something;
- **crew:** you sent it to them (see [bigger changes](#bigger-changes)).

## Who answers

A fresh agent, per thread, from the conversation engine: its first prompt is its role and `ws show t5`, which gives the thread, the document, what it's for, who wrote it, its threads and the decisions about it ([information graph](01-information-graph.md)). Its session carries the thread from one turn to the next.

It doesn't resume or fork the crew that wrote the document. What the author knew that matters should be in the graph: its handoff, its decisions, the stage it worked on. Where it isn't, the thread agent says what it can't tell, and that's a gap in the graph to close.

## Suggestions

A suggestion replaces text in the document's markdown:

```json
{ "from": "1. Ship behind a flag on Friday.", "to": "1. Ship behind a flag on Tuesday.", "version": "dcf9d7a5559d", "by": "agent" }
```

- **The agent suggests** with `ws thread suggest t5 --from - --to -`: source text, which must appear exactly once in the document at that version. A thread has one current suggestion; a newer one replaces it.
- **You suggest** from the comment box: select text, choose *Suggest an edit*, and type what it should read. When the selected text appears once in the markdown as it is (no formatting in between), that's the suggestion. When it doesn't, it's posted as a *Change* thread for the agent to turn into one.
- **Shown** in the thread as before and after, rendered, and in the document as a tracked change: the text it replaces struck through (a highlight), with a toggle to preview the document with it applied.
- **Accept** (`a`) writes it into the document, if `from` is still there exactly once and the file has no uncommitted changes:
  - in a capsule, it commits just that file, as you: `Accept suggestion on proposals/token-cache.md: ship on Tuesday`;
  - in the mission's own files, it writes it;
  - on a default branch (`.ground`), there's nothing to write to: the agent doesn't suggest there, and offers to send it to crew.
  
  Accepting resolves the thread and offers to record what was decided ([decisions](02-decisions.md)).
- **Decline** (`d`) keeps the thread open, for you to say why.

## Bigger changes

"Rewrite the rollout section" or "this needs a benchmark first" is work, not an edit. A thread (or all open *Change* threads on a document) can be **sent to crew**:

- resuming the crew that wrote the document, if it's ended and can be resumed, with "address threads t5, t7 on <doc>";
- otherwise dispatching fresh crew into the document's capsule, with `--read <doc>` and the threads.

The thread shows "c4 is on it". Crew read threads with `ws show t5`, answer in them (`ws thread reply t5 "…"`), and when they've changed the document, say so (`ws thread changed t5 "what changed"`), which puts it back with you to look at and resolve. The thread agent doesn't run while crew have the thread.

## The document

The bar above the document says where it stands: "3 open threads · 1 suggestion waiting for you", and offers **Approve**.

- **Approve** marks the document approved at its current version. It's refused while *Change* threads are open: resolve them, or decline their suggestions and resolve.
- **When the document is what the current stage made** (its artifact links to the stage), approving offers to accept the stage too, with "approved <doc>" as the reason. That's the stage's check-in done from where you read it.
- **Open *Change* threads block accepting the stage** they're on (its documents), as a review asking for changes blocks a capsule's. Questions don't.
- **What changed since you last approved or read it** is marked with bars in the margin, as built.

The orchestrator isn't in a thread's loop. It sees threads and their states through `ws show`, so it knows a stage is waiting on your threads and doesn't accept it or dispatch around them.

## Whose court

Threads are part of a mission's court:

| Headline | Court | When |
|---|---|---|
| `… answering` "answering 2 comments on proposal.md" | agent's | a thread agent is running |
| `↩ replies` "2 replies on proposal.md" | yours | threads waiting for you: an answer, a suggestion, or crew saying they've changed it |
| `✓ approve` "proposal.md has no open threads: approve it?" | yours | the current stage's document, nothing open, not yet approved |

`⏎` in the deck opens the document in the web UI at the first thread waiting for you. A thread coming back to you notifies, as crew asking you something does.

## The page

As built for reviewing documents, changed where reviews were:

- **Posting:** select text and `c`, or `j`/`k` and `c`; choose the kind (`tab`); `⌘⏎` posts. There's no draft or submit.
- **A thread in the margin** shows its kind, who it's waiting on, the conversation (the agent's reply streaming in as it writes), and a suggestion with *Accept* and *Decline*.
- **Keys:** `n`/`p` next and previous thread, `⏎` reply, `a` accept, `d` decline, `x` resolve or reopen, `i` show or hide what's changed, `A` approve the document.

## CLI

| Command | For |
|---|---|
| `ws show t5` | Anyone: the thread, the document, and links. |
| `ws thread list [--doc <ref>] [--open]` | Anyone: a mission's threads. |
| `ws thread reply t5 "…"` | Crew, and the thread agent. |
| `ws thread suggest t5 --from - --to -` | The thread agent (and crew). |
| `ws thread changed t5 "what changed"` | Crew who've changed the document for a thread. |
| `ws thread send t5 [t7…]` | You: hand threads to crew (the web UI's *Send to crew*). |
| `ws thread resolve t5` | You: resolving drafts a decision from it (see [Decisions](#decisions-from-threads)). |

The thread agent is allowed `reply`, `suggest`, `ws show` and reading; nothing else.

## Storage

Threads are conversations: `.ws/missions/<slug>/conversations/t5.jsonl`, the anchor on the first line; sending one to crew, crew saying they've changed it, and a decision drafted from it are lines there too. A document's kept versions are `docs/<version>.md`. A document's approvals are lines in `conversations/approvals.jsonl` (read on every deck refresh, so not the log), said in the mission's log, and a line in its `ws show`.

## Moving from document reviews

What was built for reviewing documents is test data only, so nothing is migrated:

- **Kept:** anchoring (`web/src/annotate/anchor.ts`), the margin and its layout, highlights, the change bars and diff, kept versions, `ws show`-style rendering of quotes and lines.
- **Replaced:** a review's `doc`, `AddDocNote`, `SubmitDoc`, blocking and re-review for documents, the `doc-review` API and the `review-doc` headline. Capsule review keeps all of its own.

## Phases

1. **Live threads, no agent:** post, reply, resolve; kinds; the page without submit. Threads stored as conversations.
2. **The thread agent** answers *Ask* and *Change* threads, from `ws show t5`. Court headlines for answering and replies.
3. **Suggestions:** the agent's and yours, shown as tracked changes, accepted and declined.
4. **The document:** approve, accept the stage from it, *Change* threads blocking the stage, decisions on resolving, sending to crew.

## Decisions from threads

Resolving a thread something was said in (by the agent or crew, or an edit accepted) has its agent draft what was decided, in its session, told what became of the thread's edits: `What: …` and `Why: …`, or "Nothing decided". The draft shows on the resolved thread, unfolded, with *Record* (edit it first, or not) and *Skip*; `n`/`p` go through drafts as well as open threads. Recording makes a decision by you, from the thread, about the document and the stage it was made for. A resolved thread with no draft offers *Record what was decided*, in your own words.

## Decided

- **Threads, not reviews,** for documents. Code review stays as it is.
- **A fresh agent per thread,** starting from the graph.
- **The agent suggests; it never edits.** Every change to the document is one you accepted, or crew made for a thread you sent them.
- **Accepting commits, as you,** when the document is in a capsule.
- **A thread with crew comes back to you** when they say they've changed the document (`ws thread changed`), or when they end or report done without saying so. Crew replying in it doesn't.
- **Approving is offered as a headline only for the current stage's documents** you've commented on (every thread resolved, not approved as they are now), and only while no crew are working on the mission. Accepting the stage from the approval is offered on the page, not done for you.

## Open questions

- **Threads on the brief.** The brief is the document most worth commenting on before crew start, but it isn't an artifact and changing it changes the plan. Threads on it probably suggest edits like any document's, with accepting one going through `ws mission brief --set`. See [discovery](05-discovery.md).
- **Crew opening threads.** Crew could ask you about a passage with a thread (`ws thread new <doc> --quote "…" "…"`) rather than `ws mission ask`. Natural, but wait until threads are in use.
- **Several suggestions in one thread,** for "here are two ways to say it". One for now.
