# Discovery

Status: building. Workspace conversations (phase 1) are in, proposing decisions and missions (briefed, lifting their capsules, or as ideas a briefing agent starts from the conversation); decisions confirmed before there's a mission are kept with the conversation and move to the mission it becomes. A mission's *Talk* tab (phase 2) is in too: its conversations (`t<n>`, numbered with its threads; `ws talk -m <slug>`) start from the mission and propose decisions (the mission's at once), edits to the brief (shown as what changes, accepted only if the brief hasn't moved since), crew to start (as you; queued where crew can't start) and follow-up missions (raised from it, linked both ways). The deck's mission view lists them. Briefing is a conversation too (phase 3): ⏎ on a mission to brief in the deck, `n` there, or *Brief it* on its page opens its briefing conversation (`t<n>` with the goal `brief`), whose agent ([briefing]'s harness, if it can hold a conversation) reads, asks, and proposes the brief with its title and capsules; confirming writes it, names the mission, lifts the capsules and makes it ready. While it's open the mission's headline says whose turn it is. `[briefing] in = "terminal"` keeps the briefing agent in a terminal, and `L` in the deck still starts one. Workspace conversations: `ws talk`, `ws conversation list|resolve|reopen`, `ws show w3`, the web UI's *Conversations* page and chat view, and `T` in the deck. The agent reads the repos (read-only git in each .ground and capsule), the missions and GitHub. Part of [context and conversation](README.md). Builds on [conversations](03-conversations.md).

## Why

Work starts before a mission. "What would a token cache take?" "Why is billing slow on Mondays?" "Should this be one service or two?" That's discovery: reading code, comparing options, asking and being asked. It's conversational by nature, and ws has nowhere for it. Today it happens in an agent session in some terminal, and what's learned stays there. The mission that follows starts from a sentence, and its briefing agent rediscovers what the conversation already knew.

The same happens alongside missions: "should stage 2 be split?", "is the proposal still right now we know about the rate limit?". Thinking about where a mission is going isn't crew's job (they're doing it) or the orchestrator's (it decides what's next, not whether the plan is right). It's yours, and it's better done talking.

## What it is

[Conversations](03-conversations.md) anchored to the workspace, or to a mission.

| | Workspace conversation | Mission conversation |
|---|---|---|
| **About** | Anything: a problem, an idea, a question about the code. No mission yet. | Where a mission is going: its plan, its brief, what to do next. |
| **Ref** | `w4` | `t4` (in the mission, sharing ids with its threads) |
| **Where** | The web UI's *Conversations* page. | The *Talk* tab on the mission's page. |
| **Reads** | Every repo's default branch and capsules, `ws status`, `ws show` on any mission, GitHub (read-only). | The same, starting from the mission: `ws show`. |
| **Proposes** | Decisions, documents, missions. | Decisions, edits to the brief, dispatches, follow-up missions. |

Proposals are confirmed by you in the page and done through ws's verbs, as you ([conversations](03-conversations.md#what-the-agent-may-do)). A discovery conversation never changes code, the brief, or a mission's course by itself.

## Ending in a mission

What discovery is for, most often. Two ways, both proposed by the agent (or asked for: "make this a mission"):

- **A mission, briefed.** The agent drafts the title, the brief (goal, context, constraints, acceptance, and stages if the work has them, as the briefing agent writes them today) and the capsules it needs. The draft shows in the conversation, rendered; you can comment on it there, as on any document, before confirming. Confirming creates the mission, ready, with its brief, and lifts its capsules.
- **An idea, for later.** A mission waiting to be briefed, as `ws mission new "…"` makes today, with the conversation attached. It sits in the deck's *Up next*, as missions to brief do now. There's no separate list of ideas: an idea is a mission nobody's briefed yet.

Either way, the mission records the conversation it came from. Its `ws show` links to it, and crew and the orchestrator read the conversation's summary (not its transcript) when they want to know how it started. Decisions made in the conversation move to the mission, keeping `from: w4`.

## Briefing becomes a conversation

Briefing is already a conversation with a goal: the briefing agent reads, asks you about scope, writes a brief, creates capsules and stops. It runs in a terminal today because that's where agents ran.

As a conversation, briefing is a mission conversation whose goal is the brief:

- `ws mission new "fe issue 45"`, or `⏎` on a mission to brief in the deck, opens it in the web UI.
- The agent reads and asks, as now. It proposes the brief (rendered, commentable) and the capsules. Confirming sets the brief, lifts the capsules and marks the mission ready, which is `ws mission ready` today.
- A discovery conversation that becomes an idea, and later a briefed mission, is one conversation throughout: the briefing picks up where the discovery left off.

The terminal briefing agent stays while harnesses that can't run conversations headless are in use.

## A mission's *Talk* tab

For thinking about a mission while crew get on with it:

- **Its plan:** "Is stage 2 still right, now we know about the rate limit?" The agent can propose an edit to the brief, shown as a tracked change on the brief and applied with `ws mission brief --set` when you accept it.
- **What next:** "Should we spike Redis before deciding?" The agent can propose a dispatch, which you confirm (or queue).
- **What was decided:** decisions proposed in the conversation are the mission's once confirmed.
- **Follow-ups:** separate work worth doing becomes a new mission (or an idea), linked back.

It's not a way to talk to crew. Crew have their terminals, and threads on documents for questions about what they've written.

## Whose court

None. Discovery is something you come back to when you want to: a reply waiting there shows as a count on the conversation, in the web UI's sidebar and the mission's *Talk* tab, and doesn't make a headline or ring the deck. A briefing conversation waiting for you is the exception: the mission can't start until you answer, so it's in your court as briefing is today.

## Where it shows

- **Web UI:** a *Conversations* page in the sidebar: each workspace conversation with its title (the agent names it after the first turn), its summary's first line, and what came of it ("→ mission token-cache", "2 decisions"). *New conversation* asks "What are you thinking about?"
- **Missions' pages:** the *Talk* tab, and in *Summary*, the conversation the mission came from.
- **The deck:** `T` opens the *Conversations* page; a mission's view lists its conversations. No chat in the terminal.
- **The CLI:** `ws talk ["first message"]` starts one and opens it; `ws conversation list`; `ws show w4`.

## Storage

Workspace conversations: `.ws/conversations/w4.jsonl` (turns, session, summary, outputs) and `.ws/conversations/w4/` for what it writes (sketches, a draft brief). A mission's: in the mission ([conversations](03-conversations.md#storage)).

## Phases

1. **Workspace conversations:** the *Conversations* page, reading the workspace, proposing decisions and missions (briefed, or as ideas).
2. **A mission's *Talk* tab:** edits to the brief as tracked changes, dispatches, follow-ups.
3. **Briefing as a conversation,** for harnesses that can run conversations; the terminal briefing agent for the rest.

## Decided

- **Ideas are missions waiting to be briefed,** with the conversation attached. No second backlog.
- **Discovery has no court,** except briefing, which holds a mission up.
- **Read and propose.** Missions, brief edits, dispatches and decisions happen when you confirm them.
- **Workspace conversations have their own refs** (`w4`), so a ref means the same thing wherever it's read.

## Open questions

- **Workspace decisions.** "No Redis anywhere" outlives a mission. Until missions' decisions show what recurs, a workspace conversation's decisions stay with it and move to the mission it becomes. See [decisions](02-decisions.md).
- **Exploring costs.** A good answer to "why is billing slow?" may mean reading a lot of code. The agent should say what it's about to explore when it's a lot, and the per-run limits apply.
- **Sharing.** A discovery conversation worth showing a teammate would want exporting (to a doc, or the records repo). Later.
