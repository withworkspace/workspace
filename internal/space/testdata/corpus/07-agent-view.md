# The agent view

Status: Phase 1 built on `v2`, 2026-11-10, and tried on real crew (Claude Code 2.1, haiku). See [What Phase 1 built](#what-phase-1-built) and [What trying it taught](#what-trying-it-taught). Phases 2 and 3 are still proposals. It builds on the crew transcript (`270394d`) and the web UI's conversation view (designs/03).

Today a crew's page opens on its terminal and offers its transcript as a second tab. This design makes the transcript into the place you meet an agent. You read what it's doing, answer what it asks, approve what it wants to run and tell it what's next, all without the terminal. The terminal stays one click away, for anything the view can't do.

## Why

The terminal is faithful, and it's how the deck works, but in a browser it's a poor way in:

- **You can't read back.** It's drawn screen by screen with no scrollback: what crew did ten minutes ago is gone. You also can't link to it, search it or select across it.
- **It's the harness's UI, not ours.** Permission prompts, questions and plans are boxes drawn in characters. ws knows (through its hooks) that crew are waiting, but not what for, so the mission page can only say "go to the terminal".
- **It doesn't fit a phone,** and can't: crew draw at their own size.
- **It isn't the rest of the app.** A conversation (w1, t1) is turns, steps, proposals and a composer, and a crew is a screen. They're both agents.

The transcript is already most of the way. It shows what crew were sent, what they said and every step, and it reads well. But it can only be read: it's polled every four seconds, you can't answer from it, and it doesn't say what's happening now.

## What "the way in" needs

Each item is something you'd otherwise open the terminal for.

1. **Live.** A new step or message shows within a second, not four. While crew work, the step in hand shows as running: "running `go test ./...`", from the last tool call with no result yet.
2. **Talk to it.** A composer, as in a conversation:
   - Running crew: what you write is typed in and sent, as the deck does (`Paste`, then Enter, recording `TalkedTo` so the end of its turn is your court).
   - Crew that have ended: the message resumes them (`resume` with it).
   - A draft is kept per crew.
3. **Interrupt.** "Stop this turn" (Escape), as distinct from stopping the crew.
4. **Permissions.** When crew stop to ask (`activity = permission`), the view shows what for:
   - The pending tool call, from the transcript: "c1 wants to run Bash: `rm -rf build/`".
   - Its buttons: Allow once, Always allow this, Deny and say why. Each types the harness's keys.
   - The mission page's headline gets the same card, so you can approve without leaving it.
5. **Questions.** When crew use their question tool (`activity = question`, `Notice` and `Options` from the hook), the view shows the question with a button per option and a box for your own answer, which types the harness's keys.
6. **Plan.** Claude Code's TodoWrite calls are its plan: the latest one, pinned at the top as a checklist (done, in hand, to do), says where it's got to at a glance.
7. **Edits as edits.** An Edit or Write step shows what changed (its old and new text, as a small diff) and links to the file in the capsule's file explorer, at the line.
8. **Subagents folded.** Task steps fold into one, with what came back.
9. **Where it stands, on top.** A slim header gives its status (working, waiting for you, permission), what it's working in, how long it's been at it, and the last notice.

Everything else (a slash command, a TUI menu ws doesn't know, anything odd) is the terminal's, one tab away. It's also the fallback whenever the view can't tell what the screen is asking.

## How

### Phase 1: the same crew, a better view (no change to how crew run)

**Server**

- `GET .../crew/{id}/transcript/stream`: an event stream that reads the session file once and then follows it, sending entries as they're written. It remembers its offset, waits on file changes, and handles a line half-written. The same parser serves both (`harness.ReadTranscript`), extended to keep:
  - the tool-use id;
  - whether a call is still pending (no result yet);
  - the line's time;
  - a call's input as data for the tools the view draws specially (Edit, Write, TodoWrite, Bash, Task, the question tool), clipped.
- `POST .../crew/{id}/say {text}`: types a message into running crew (paste, Enter, `TalkedTo`), or resumes ended crew with it. One endpoint, so the page doesn't decide.
- `POST .../crew/{id}/answer {permission: allow|always|deny} | {option: n} | {text}`: types the harness's keys for the prompt on screen. The keys are per harness (`harness.PromptKeys`), checked against the screen before typing: if the screen isn't showing the prompt ws expects, it refuses and says to use the terminal. A wrong guess must never approve something.

**Page**

- The crew page opens on the view (it's renamed *Conversation*) whatever the crew's state. The terminal is the second tab.
- The view is the conversation component (designs/03), shared with w1 and t1: turns, steps, a composer. On top of that it adds the plan, the pending prompt card and the live step.
- The mission page's headline for a crew permission or question shows the same prompt card.

**Harnesses**

| | Transcript | Say | Prompts |
|---|---|---|---|
| Claude Code | session file | paste + Enter | permission menu, AskUserQuestion |
| pi | session file | paste + Enter | its own menu: keys to learn |
| Cursor | none | paste + Enter | terminal only |

With Cursor, the view is its terminal's screen plus the composer, and says why.

### Phase 2: a structured channel (Claude Code), if Phase 1 earns it

Claude Code can run headless over a stream. `-p --input-format stream-json --output-format stream-json` sends it messages as JSON and gets its events back as they happen, partial text included. `--permission-prompt-tool` makes permission prompts a call to an MCP tool. If ws served that tool, permissions and questions would come to ws as data, with nothing to read off a screen and no keys to guess.

This changes what crew are. A crew run this way has no TUI, so the deck's zoom-in would have to draw it from the stream: the same view, in the terminal. It's the honest end state, but it's a large change to the deck. It should follow only if Phase 1 shows the view is where people want to be, and the key mapping proves fragile.

### Phase 3: everywhere

- A crew's headline ("c1 asks", "c1 needs permission") opens the view at the prompt.
- ⌘K gains "Talk to c1".
- A crew's hover card offers it.
- The conversation component becomes the one way the web UI shows any agent: workspace conversations, threads, crew, and in time the orchestrator's runs.

## Open questions

- **Prompt keys.** Claude Code's permission menu and its question UI change between releases. Reading the screen before typing makes a wrong guess fail closed, but it needs tests against real sessions per harness version. Note: does the question tool take a number, or arrows and Enter?
- **Long messages.** Pasting multi-line text into a TUI depends on bracketed paste. The deck already pastes; check that a long message lands as one.
- **Partial output.** A session file gets a message once it's written, so in Phase 1 a long reply appears whole, not word by word. The live step covers the wait. Phase 2 would stream.
- **Trust.** Approving a permission from a page is the same trust as pressing the key in the terminal, from the same signed-in browser. Should "Always allow" be offered from the web at all, or only once?
- **Resuming to talk.** Resuming ended crew with a message costs a fresh process and its context. Should the composer say so before sending?

## What Phase 1 built

- **The view.** The crew page opens on *Conversation* (`mission/AgentView.tsx`); *Terminal* is the second tab (`t` switches, `?view=terminal` links to it). It follows the session over a stream, `GET .../transcript/stream`, which sends the transcript again whenever the file changes (looked at every 400ms, a stat). It shows:
  - turns, with ws's brief to the crew folded and harness notices ("Interrupted") as notices;
  - steps, the one in hand spinning;
  - edits as diffs, with a link to the file in the capsule explorer;
  - the plan (TodoWrite) pinned on top, with its progress;
  - questions as their options.
- **Entries as data.** `harness.ReadTranscript` keeps each line's time, a call's id and whether it's still pending. It keeps edits, todos and questions as data, and leaves subagents' sidechain lines to their Task call.
- **Talking.** `POST .../crew/{id}/say` pastes your words into running crew and sends them (`TalkedTo`, logged), or resumes ended crew with them. The composer keeps a draft per crew, and while crew work it offers Interrupt.
- **Answering.** `POST .../crew/{id}/answer` takes a permission (allow, always, deny), one of a question's options, your own words, or an interrupt.
  - It reads the menu off the screen first (`harness.ReadPrompt`). It refuses, typing nothing, unless the screen shows the prompt crew's hooks reported: a permission for `permission`, or the hook's options for `question`.
  - It moves the cursor with arrows and presses Enter, rather than typing digits.
  - "Always allow" asks you to confirm first.
  - `GET .../crew/{id}/prompt` returns the menu as read, so the card can show what's being asked even before the transcript has the step.
- **The mission page.** A crew's permission or question headline shows the same card, answered in place. A crew waiting for you gets a reply box.

## What trying it taught

- **The transcript can lag the screen.** Claude Code wrote both tool calls of a parallel pair to its session file only once the permission prompt had been up a while, so the card first had nothing to show. Reading the menu's context off the screen (`/prompt`) covers that gap. A single call was in the file at once.
- **Not every menu is numbered.** Permissions and questions are (`❯ 1. Yes`). The folder trust dialog isn't (`❯ No, exit` / `Yes, I trust this folder`), and `ReadPrompt` doesn't read it, so the terminal answers trust. It's asked once per repo.
- **Crew may run in auto mode,** in which permissions don't come up at all. Testing permissions took switching the crew to manual (shift+Tab, from the terminal).
- **Menus change shape.** Claude Code's permission menu had four options ("switch to auto mode" is new), one of them wrapped over two lines, and the last line was drawn over by its status bar ("4. Nount-the-greetings"). Matching option text by its start ("Yes", "No", "don't ask again" / "always") held up; a fixture of that screen is in `internal/harness/testdata`.
- **Answers landed every time** in the live test: a question's option, allow, deny, a message to running crew, and resuming ended crew with one. Interrupt sent Escape, but the reply it was meant to stop had already finished. It's the terminal's own key, so it should behave as Escape does there.

## Recommendation

(Written before Phase 1; kept for the reasoning.) Build Phase 1 in this order:

1. The stream and the richer entries.
2. The view on the shared conversation component, opening first.
3. `say`, with Interrupt.
4. The plan and edit steps.
5. Prompts, Claude Code first, with the screen check.

Try it on real crew in the test workspace (haiku). Then decide on Phase 2 from what the key mapping costs to keep right.
