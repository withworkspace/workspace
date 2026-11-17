# Decisions

Status: building. Decisions, `ws mission decide`, their views and the orchestrator's prompt (phase 1), decisions from answered flags (phase 2) and from resolved threads (phase 3, see [document threads](04-document-threads.md#decisions-from-threads)) are built. Part of [context and conversation](README.md). Builds on the [information graph](01-information-graph.md).

## Why

The most useful thing a new agent can know about a piece of work is why it's like that: "the cache is keyed per tenant, because tokens are issued per tenant and per-user keys mint one per pod". It's also the least written down. Today a reason ends up in one of:

- a note in the log (`ws mission note`), of which agents see the last ten;
- a crew's handoff, under *Learned*;
- a review comment's thread, or a flag's answer;
- a crew's or the orchestrator's conversation, which ends with it;
- nowhere.

So crew re-litigate settled questions, the orchestrator dispatches work the user ruled out, and a new agent answering a comment can't say why the document says what it says.

## What a decision is

A short statement of what was decided, why, by whom, where it was decided, and what it's about.

```json
{
  "id": "d1",
  "what": "Key the token cache per tenant, not per user.",
  "why": "Tokens are issued per tenant; per-user keys would mint one per pod.",
  "by": "you",
  "from": "t2",
  "about": ["records/token-cache:proposals/token-cache.md", "s1"],
  "at": "2026-11-09T19:10:00Z",
  "state": "current"
}
```

| Field | Meaning |
|---|---|
| `what` | The decision, as an instruction or a fact. One sentence. |
| `why` | The reason, in a sentence or two. Optional, but the point. |
| `by` | `you`, a crew id, or `orchestrator`. |
| `from` | Where it was decided: a thread, a conversation, a flag, a review comment, a crew. |
| `about` | Refs it bears on: documents, stages, capsules, other decisions. |
| `state` | `current`, `superseded` (by a later decision, named in `supersededBy`), or `withdrawn`. |

It isn't an ADR. ADRs are documents, shared through a records repo, reviewed and kept. Decisions are ws's working memory of a mission: small, many, and local. One can be written up as an ADR (crew do that, and the ADR artifact links back).

## How decisions are made

Where the deciding already happens, so that recording one is a step, not a chore:

| Where | How |
|---|---|
| **Resolving a thread** | Resolving offers "Record what was decided", filled in by the thread's agent from the conversation (what and why). You edit it or skip it. |
| **Answering a flag** | Your answer to the orchestrator's question is a decision, recorded with the question as `why` unless you skip it. |
| **A conversation** | The agent proposes one ("Shall I record: no Redis, an in-process cache is enough for now?"); you confirm. See [conversations](03-conversations.md). |
| **Crew** | `ws mission decide "<what>" --why "<why>" [--about <ref>]...` for decisions within their task: an approach taken, an option ruled out. Their handoff's *Learned* can point at them. |
| **The orchestrator** | The same command, for decisions about the mission's course. It already notes what it decided each run; a note that's a decision becomes one. |
| **You, anywhere** | `ws mission decide`, or *Decide* in the web UI's mission page. |

Crew and the orchestrator may record decisions within their part. Decisions about scope and direction are yours: they come from your answers, your threads and your conversations.

## Changing your mind

A decision is never edited. A new one supersedes it (`ws mission decide "…" --supersedes d1`), or it's withdrawn (`ws mission decide --withdraw d1 "why"`). Superseded and withdrawn decisions stay, out of the way: views show current ones, and `ws show d1` shows what replaced it.

## Where they show

- **`ws show` of anything a decision is about** lists the current ones (see the [example](01-information-graph.md#ws-show)).
- **The mission's view** lists its current decisions, newest first, capped.
- **Crew** are told to read the decisions about their stage before starting (`ws show s2` lists them), and not to go against one without asking.
- **The orchestrator** sees current decisions in its prompt: they're short, and they're what it most needs to respect.
- **The web UI** shows them on the mission page (a *Decisions* section), and beside a document's threads.
- **The deck** shows them in the mission view's log, as events.

Rendered for agents, a decision is one line:

```
d1 (you, from t2): Key the token cache per tenant, not per user. Why: tokens are issued per tenant; per-user keys would mint one per pod.
```

## Storage

With the mission: `.ws/missions/<slug>/decisions.jsonl`, append-only. Each line is a decision made, superseded or withdrawn; the current state is worked out from them. Making one also logs a `decision` event, so it shows in the log and the deck.

## Phases

1. **Decisions and `ws mission decide`**, with views in `ws show` and `ws mission show`, and the orchestrator's prompt.
2. **From flags:** answering one records a decision.
3. **From threads:** resolving one offers to record what was decided. (With [document threads](04-document-threads.md).)
4. **From conversations:** the agent proposes, you confirm. (With [conversations](03-conversations.md).)

## Decided

- **Decisions are mission-scoped.** A conversation before any mission keeps its decisions with it, and they move to the mission it becomes. See [discovery](05-discovery.md).
- **Never edited, only superseded or withdrawn.** What was believed when matters to anyone reading the history.
- **Agents may decide within their part.** Recording isn't the same as deciding: a decision crew made silently is still made, and better written down.

## Open questions

- **Workspace-wide decisions** ("no Redis anywhere", "new services are Go"). Some decisions outlive a mission. They might belong in the records repo as ADRs, with ws reading them; or a workspace-level `decisions.jsonl`. Wait until missions' decisions show which ones recur.
- **Agents proposing decisions they shouldn't make.** If crew start deciding scope, `ws mission decide` could refuse `by` crew on certain kinds, or flag the user. Watch for it first.
