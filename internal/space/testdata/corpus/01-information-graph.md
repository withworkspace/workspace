# Information graph

Status: building. `ws show` for every kind of thing a mission has (phase 1), the artifact → stage link, document history and handoff mentions (phase 2), prompts pointing at `ws show` (phase 3, the orchestrator's inline context unchanged for now) and `scripts/coldstart.sh` (phase 4) are built. Part of [context and conversation](README.md).

## Why

An agent in a mission should be able to start from one thing (a document, a thread, a stage, a crew) and find out what it is, why it's like that and where it's at, without being told everything up front.

Today, what an agent knows is what its prompt pastes in:

- **Crew** get the brief, their stage or dispatch, "where earlier crew left off", and are told to run `ws mission show -f llm`.
- **The orchestrator** gets a prompt assembled per run from nearly twenty inputs: the brief, stages, capsules, artifacts, the log's tail, the crew's output and handoff, reviews, where the mission stands.
- **`ws mission show -f llm`** is one page: the mission, its capsules and default branches, stages, artifacts, the brief and the last few log lines.

That works for a crew that's about to do the next stage. It doesn't for a question about one thing. For the proposal in `records/token-cache`:

| Question | Where the answer is today |
|---|---|
| What is it for? | The stage that produced it, if you match the stage's text to the file by eye. Nothing links them. |
| Who wrote it, and why like this? | The artifact records the crew (`c1`). Why is in c1's handoff, its conversation, or nowhere. |
| What's changed since I last read it? | `git log` in the capsule, if you know to look. |
| What's been said about it? | Review comments, log notes, flags and their answers, each in its own place. |
| What's been decided? | Notes in the log, of which agents see the last ten. |

Each role's prompt works around this by pasting more. Prompts grow, roles duplicate each other's assembly, and what isn't pasted is out of reach.

## The idea

Treat what ws knows as a graph: **things** with **refs**, and **links** between them. Give agents one way to look at any thing, with its links as refs to follow: `ws show <ref>`. Prompts shrink to a role and where to start.

## Things and refs

Most things already have a ref. A ref is read in the current mission (the `-m` flag, `$WS_MISSION`, or the capsule you're in), as other mission commands are.

| Thing | Ref | Has one today? |
|---|---|---|
| The mission | (none: `ws show` alone), or `-m <slug>` | yes, as a slug |
| A stage | `s2` | new |
| A capsule | `demo/token-cache` | yes |
| Crew | `c1` | yes |
| A document or other artifact | `records/token-cache:proposals/token-cache.md`, `mission:notes/x.md`, `demo#12`, a URL | yes |
| A handoff | `mission:handoffs/c1.md` | yes, as an artifact |
| A review of code, a comment on it | `r1`, `r1.c2` | yes |
| A thread on a document | `t3` | new: [document threads](04-document-threads.md) |
| A conversation in a mission | `t4` | new: [conversations](03-conversations.md). A thread is a conversation, so they share ids. |
| A conversation about the workspace | `w4` | new: [discovery](05-discovery.md). Not in a mission, so it has its own prefix. |
| A decision | `d2` | new: [decisions](02-decisions.md) |
| A repo | `demo` | yes |
| An orchestrator run | its id, from when it started: `20261109-150405` | yes (the latest few are kept) |

Log events aren't things: they show up in the views of the things they're about.

## Links

A link is recorded when the thing happens, if it can't be worked out later; otherwise it's worked out each time, as ws already does for a mission's state and its artifacts' state.

| Link | Recorded or worked out | Today |
|---|---|---|
| artifact → crew who recorded it | recorded | yes |
| artifact → stage it was made for | recorded: the stage current when it was recorded, or the crew's stage | **new** |
| crew → stage, capsule, dispatch, handoff | recorded | yes |
| crew → artifacts it recorded, threads it answered, decisions it made | worked out (from those things' records) | **new** view |
| document → versions, and the commits that made them | worked out: `git log` on the file in its capsule, or its kept versions | **new** view |
| document → threads, reviews, decisions about it | worked out (from their records) | **new** |
| thread → document, quote, lines | recorded | yes, as doc review comments |
| decision → where it was made, what it's about | recorded | **new** |
| stage → crew, artifacts, decisions, acceptance and why | worked out, mostly from the stage record | partly |
| mission → the conversation it came from | recorded | **new**: [discovery](05-discovery.md) |
| handoff → artifacts and refs it mentions | worked out: refs in its text | **new** |

## `ws show`

```
ws show [ref] [-m <mission>] [-f text|llm] [--full]
```

With no ref it shows the mission, the index of everything else. Every view has the same shape, so agents learn it once:

1. **What it is**, in a line: its kind, ref, name, and where it stands.
2. **The essentials:** what you'd want before deciding whether to read further. For a document: what it's for, its state, size, and what changed lately. For a thread: the quote, the conversation so far (summarised if long), and its state.
3. **Links:** one line each, with the ref to follow and why you might. Lists are capped ("4 more: `ws show <ref> --links threads`").
4. **What happened lately:** log events about it, newest last.
5. **More:** how to read it whole (the file's path, `--full`).

For example, `ws show records/token-cache:proposals/token-cache.md -f llm`:

```
Document records/token-cache:proposals/token-cache.md — "Token cache proposal" (proposal, draft: in capsule records/token-cache, no PR yet)

For stage s1 (current): Write a proposal for the cache (check in with the user after it).
Written by c1 (ended, reported done). 2 versions: 275b7dd "key by tenant, and say what happens when minting fails", 9e04c1a "Proposal: a token cache".
Read it at: …/repos/records/token-cache/proposals/token-cache.md (62 lines)

Links:
  → c1               wrote it: its dispatch, handoff and what it learned    ws show c1
  → s1               the stage it's for, and what's left in it               ws show s1
  → t2 (resolved)    "Every request pays for a token…": you asked why per user; decided d1   ws show t2
  → t5 (open)        "on Friday": you asked for a change; a suggestion waits for you      ws show t5
  → d1               Key the cache per tenant, not per user                  ws show d1

Lately:
  19:08 you commented on "on Friday" (t5)
  19:09 the thread agent suggested "on Tuesday" (t5)
```

`-f text` is the same for people, with colour. `--full` includes everything a view caps or summarises.

### Size

Each view has a budget (about 4 KB for `-f llm`), and says what it left out and how to get it. An agent that needs the whole transcript of a conversation, all forty log events, or the document itself, asks for it. That's what "everything it needs, when it needs it" means in practice: the first view is enough to decide where to go next.

### The mission is the index

`ws mission show -f llm` stays, and becomes the mission's `ws show`: what it is, where it stands, its stages, then links to everything else. Every line gains the ref to follow.

## Prompts become entry points

| Role | Today | With the graph |
|---|---|---|
| Crew | The brief pasted, its stage or dispatch, where earlier crew left off, how to report. | Its role, its task (stage or dispatch), the brief (still: it's the one thing every crew reads), and `ws show s2` as where to start. |
| Orchestrator | Assembled from nearly twenty inputs. | What triggered it, the brief, where the mission stands, and `ws show` for the rest. Inline what every run needs, link what only some do. |
| Answer role | The question, and `ws mission show -f llm`. | The question, and `ws show` for the flag it answers. |
| Thread agent | — | Its role, and `ws show t5`. |
| Conversation | — | Its role, its anchor, and `ws show t4`. |

Inlining stays where it's always needed: the orchestrator mustn't need three tool calls to know what just happened.

## Cold-start tests

The graph is good enough when a fresh agent, given only a ref, can answer questions about it. So that's the test:

```
scripts/coldstart.sh <workspace> <mission> <ref> "<question>"
```

starts a headless agent in the workspace with only the role "answer from ws's records", the ref and the question, and prints its answer and which `ws show` calls it made. Run against ws-test after a mission's been through a stage and a review, with questions whose answers are in ws:

- Why does the proposal key its cache per tenant?
- What's left to do in stage 1?
- Who wrote the proposal, and what did they find hard?
- What changed in the proposal since the user first commented?
- Which comments on it are still open, and what's each waiting on?

A wrong or "I can't tell" answer means a missing link or a view that hides the answer. These run by hand (they cost tokens), when the graph changes.

## Phases

1. **`ws show`** for the things that exist: mission, stage, capsule, crew, artifact, review and comment. Refs on every line of `ws mission show`.
2. **The new links:** artifact → stage, document versions, handoff mentions, and the views that turn them around (crew → what it made, document → what's been said).
3. **Prompts:** crew, orchestrator and answer role start from `ws show`. Compare orchestrator runs before and after on ws-test.
4. **Cold-start tests** as a script, and a first set of questions.

Threads, conversations and decisions add their own views as they arrive.

## Decided

- **One command, `ws show`,** not a command per kind. Agents learn one shape.
- **Refs are read in the current mission,** like everything else in `ws mission`; no new global syntax.
- **Links are recorded only when they can't be worked out later.** The rest is worked out each time, so it can't drift.

## Open questions

- **Search.** "Has anyone decided anything about Redis?" wants `ws find "<words>"` across decisions, threads, notes and handoffs. Useful, but later: the graph should make most questions a walk, not a search.
- **Across missions.** A follow-up mission raised by crew records where it came from. Should `ws show` cross into the other mission? Probably as a link with its slug (`-m <slug>`), not by merging graphs.

## Risks

- **Views drift from what agents need.** Cold-start tests are the check.
- **Bloat.** Every view wants one more line. The size budget is a rule, not a guideline.
- **A link ws missed.** Something recorded outside ws (a file copied in by hand, a commit made without crew) has no record. Working links out where possible limits this.
