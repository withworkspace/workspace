# Conversations

Status: building. The engine (headless turns resuming a session per conversation, Claude Code) runs the thread agent and workspace conversations; the chat view (with the agent's calls shown as it goes, from its run file, and folded under each reply), titles and summaries are built, as are proposals of decisions and missions (`--propose` on `ws mission decide` and `ws mission new`, confirmed with a button under the reply, or `ws conversation accept|decline`). Mission conversations (the Talk tab) are built too, proposing edits to the brief and dispatches besides. Part of [context and conversation](README.md). Builds on the [information graph](01-information-graph.md) and [decisions](02-decisions.md).

## Why

ws has workers (crew) and a decider (the orchestrator), and treats both as structured: given a task, watched by the deck, acting through ws's verbs. What it lacks is an agent to think with. Exploring a problem, comparing options, questioning a proposal: that's conversation, and it has nowhere to happen in ws.

Two things need it now:

- **Threads on documents**: you comment on a sentence, an agent answers in the thread, you reply. See [document threads](04-document-threads.md).
- **Discovery**: before a mission exists, or about where one is going. See [discovery](05-discovery.md).

Both are the same thing: turns between you and an agent, about something, producing something. So they're built once.

## Concepts

| Concept | What it is |
|---|---|
| **Conversation** | Turns between you and one agent, about an anchor, kept with a summary and what came of it. Ref `t<n>` in a mission (threads included), `w<n>` for the workspace's. |
| **Anchor** | What it's about: the workspace, a mission, or text in a document (a thread). It decides where the conversation shows and what the agent starts from. |
| **Turn** | A message: yours, the agent's, or ws's (a note that something changed: "the document has a new version"). |
| **Run** | One headless agent run that answers your latest messages, resuming the conversation's session. |
| **Outputs** | What the conversation made, through ws's verbs: replies, decisions, suggestions, documents, missions. Each links back to it. |

## How a turn works

1. You send a message. It's stored as a turn at once.
2. If no run is going, ws starts one. Messages sent while one is going wait, and go into the next run together.
3. The run resumes the conversation's agent session, headless, with your new messages. The first run starts the session and records its id.
4. The agent's reply streams into the page as it comes. What it does on the way (reading a file, `ws show s1`) shows as quiet lines under the reply, collapsed.
5. When the run ends, the reply is stored as a turn, with anything it made (a decision proposed, a suggestion) attached.

The session holds the agent's memory of the conversation, so each run passes only what's new. ws keeps its own copy of the turns, so the conversation survives a session that's lost: a new session starts from `ws show t4`, which has the turns and summary.

### Starting from the graph

A conversation's agent doesn't get its context pasted in. Its first prompt is its role, its anchor, and where to start:

```
You're talking with the user about the document records/token-cache:proposals/token-cache.md,
in thread t5 on the text "on Friday". Start with `ws show t5`, which has the thread so far,
the document and the mission it's part of. …
```

When something changes between turns (the document has a new version, a decision is made elsewhere), ws adds a turn saying so, with the ref to look at. The agent decides whether it matters.

### Summary

After each run, the agent updates the conversation's summary: a few lines on what it's about, what's been concluded and what's open. `ws show t4` leads with the summary and the last few turns, not the whole transcript. The summary is what other agents read when they follow a link to the conversation.

## What the agent may do

Conversations change nothing by themselves. Their agent:

- **reads:** files in capsules and `.ground`s, `ws show`, `ws rg`, `ws status`, `git log/show/diff` read-only, `gh … view`;
- **answers:** its reply is its output;
- **proposes,** through verbs that record a proposal for you to confirm in the page: a decision (`ws mission decide --propose`), a suggested edit on a document (in a thread), a mission (`ws mission new --propose`), a dispatch (`ws mission dispatch --propose`);
- **writes** only to the conversation's own folder: a sketch, a comparison table, a draft brief, which you can keep as an artifact.

It never edits code or a document directly, commits, or starts crew. Proposals appear as buttons under its reply ("Record decision", "Create mission", "Dispatch crew"); confirming one does it, through the same verbs, as you.

This is the deck's principle applied to conversation: the conversation is where you think; what changes the work goes through the control plane, confirmed by you.

### Enforcing it

Like the orchestrator, runs are limited to what's allowed, refusing anything else without asking. ws keeps one list of what a conversation's agent may run (`converseCommands` in `internal/harness/converse.go`: reading, `ws show` and its kin, a thread's `ws thread suggest`, and for a talk, the missions, GitHub's views and read-only `git -C <path>` in each capsule and .ground), and each harness enforces it its own way. All three are verified against the real CLIs and hold conversations, threads included.

| Harness | Headless, limited to allowed commands | Resumes a session headless | Streams as it goes |
|---|---|---|---|
| Claude Code (2.1.319) | yes: `-p --permission-mode dontAsk --allowedTools` | yes: `--resume <id>`, keeping its id. Sessions are kept per directory, so every turn runs from the workspace root. | yes (`stream-json`) |
| Cursor (2026.11.05) | yes: `--mode ask` (no edit or write tools) and **no** `--force`, so a shell command runs only if the permissions allow it, else it's `rejected`. ws runs it in `.ws/converse/cursor/<hash>/`, one folder per allow list, whose `.cursor/cli.json` allows exactly it (`Shell(git log)` allows `git log …`, not `git status`). It still reads files anywhere, and your own global allow rules (`~/.cursor/cli-config.json`) apply on top: ws can't turn them off. | yes: `--resume <chatId>`, from any folder. A chat that's gone is quietly started afresh under its id, so ws looks for it in `~/.cursor/chats/*/<id>` first, and starts again from `ws show` if it's not there. | yes (`stream-json`, its own schema: the reply is the last assistant message, not `result`, which joins them all) |
| pi (1.5.0) | yes: `--tools read,bash,grep,find,ls`, `--no-extensions --no-mcp`, and an extension ws writes (`.ws/converse/pi/`) whose `tool_call` handler blocks any shell command off the list, chained or redirected (what a command reads on stdin, a suggestion or a brief, may come as a quoted heredoc). | yes: `--session-id <id>`, an id ws picks, starts it; `--session <id>` continues it, and fails if it's gone (`--session-id` would quietly start it empty), so ws starts again from `ws show`. | yes (`--mode json`). It waits on stdin while it's open: ws runs it with none. |

Cursor's `--force` runs anything; nothing ws runs passes it. The orchestrator is limited the same ways on Cursor and pi (to its own list, as Claude Code's is: ws's mission verbs, lift, status and show, GitHub's PR views, read-only git), and reports its calls on all three. A run file starts with a line naming its stream, so the page reads any harness's progress. Neither Cursor nor pi has a turn limit like Claude Code's `--max-turns`: a run's only bound is its time limit.

Tried on all three through ws: workspace conversations (reading git, resumed, and recovered from a lost session), a thread's suggested edit, the decision drafted when one's resolved, proposals of decisions and missions (a brief on stdin), and the page's live view. Not yet on Cursor's default model (this test machine's Cursor plan refuses it), nor on pi with providers its extensions supply.

## Configuration

A role, like crew, briefing and the orchestrator:

```toml
[conversation]
harness = "claude"
model = "sonnet"   # it's thinking with you: not the smallest model
```

Defaults to the crew's harness: Claude Code, Cursor and pi can all hold conversations. The model defaults to the harness's own choice for conversations (Claude Code's `sonnet`, Cursor's `claude-sonnet-5-5-medium`, pi's own default). Document threads and discovery can set their own (`[conversation.threads]`, `[conversation.discovery]`) if one wants a quicker model than the other.

## Where they show

- **Web UI.** Conversations are a web thing: the markdown, diagrams and documents they talk about render there, and messages link to what they're about. A chat view: your messages and the agent's, streamed; its tool use folded under each reply; proposals as buttons; the summary at the top. `⌘⏎` sends, `esc` leaves.
  - Workspace conversations: a *Conversations* page in the sidebar.
  - A mission's: a *Talk* tab on its page.
  - A document's threads: in its margin ([document threads](04-document-threads.md)).
- **The deck** lists a mission's conversations and threads in its mission view (title, state, last turn), and `o` opens one in the web UI. It doesn't host chat: the deck is the control plane.
- **`ws show t4`** for agents, and `ws conversation list` for a mission's or the workspace's.

## Whose court

A conversation is either the agent's (a run is going), yours (the agent replied and you haven't), or settled (resolved, or left).

- **Threads on a document** can hold work up (a change you've asked for, a question crew needs answered), so they're in the mission's court: "answering 2 comments on proposal.md" is the agent's; "2 replies on proposal.md" is yours. See [document threads](04-document-threads.md).
- **Discovery** is something you come back to when you want to. A reply waiting there is shown quietly (a count by the conversation), not as a headline, and doesn't ring the deck.

## Storage

`.ws/missions/<slug>/conversations/t4.jsonl` for a mission's conversations (and threads), `.ws/conversations/w4.jsonl` for the workspace's: one turn per line, plus lines for the session id, summary and outputs. Ids count up per mission (`t1`, `t2`, …) and across the workspace (`w1`, `w2`, …).

## Cost

Each turn is a run of a capable model, reading what it needs. To keep that in hand:

- One run at a time per conversation; messages sent meanwhile go into the next.
- A cap on runs going at once, per workspace (default 3); more wait.
- `--max-turns` (or the harness's equivalent) on each run, so one can't wander.
- The model is configurable per kind of conversation.

## Phases

1. **The engine:** conversations, turns and runs, with Claude Code; the chat view; `ws show t<n>`. Its first use is [document threads](04-document-threads.md).
2. **Proposals:** decisions, missions and dispatches proposed by the agent and confirmed in the page.
3. **Workspace and mission conversations** ([discovery](05-discovery.md)).
4. **Other harnesses:** Cursor and pi, verified and built.

## Decided

- **One building block.** A thread on a document, a discussion of a mission and a discovery session are conversations with different anchors.
- **Headless runs, resumed per turn,** not a terminal session: the page shows the conversation, so it can render what it's about, and nothing waits on a terminal being open.
- **Read and propose, never change.** What changes the work is confirmed by you, through ws's verbs.
- **Fresh agents, not forked authors.** A conversation's agent starts from the graph, not from another agent's session: that's what the graph is for.

## Open questions

- **Long conversations.** A session's context fills up. The summary lets a conversation start a fresh session that reads it, but when to do that (and whether the harness compacts on its own first) needs trying.
- **Talking to crew.** Crew are conversations too, in their terminals. Should a crew's terminal show as a conversation in the web UI, for asking it something without interrupting its work? Probably not: that's what threads and "send to crew" are for.
- **More than you.** If a workspace is shared, conversations have more than one person in them. Not now.

## Risks

- **A second product.** A chat UI can grow without end. The rule: conversations exist to produce structure (decisions, documents, missions), and the UI does only that well.
- **Cost.** A capable model, per turn, reading around. The caps above, and watching ws-test's usage.
- **Proposals ignored.** If confirming a proposal is a chore, decisions won't get recorded. One click, prefilled by the agent.
