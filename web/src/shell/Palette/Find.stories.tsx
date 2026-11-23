import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { Button } from "../../ui/Button";
import { HoverCardLine } from "../../ui/HoverCard";
import { Input } from "../../ui/Input";
import { FindAct } from "./FindAct";
import { FindActs } from "./FindActs";
import { FindBar } from "./FindBar";
import { FindCodeCard } from "./FindCodeCard";
import { FindCodeRow } from "./FindCodeRow";
import { FindFoot } from "./FindFoot";
import { FindFootKey } from "./FindFootKey";
import { FindItemCard } from "./FindItemCard";
import { FindList } from "./FindList";
import { FindMore } from "./FindMore";
import { FindNone } from "./FindNone";
import { FindNote } from "./FindNote";
import { FindPane } from "./FindPane";
import { FindPullCard } from "./FindPullCard";
import { FindRow } from "./FindRow";
import { FindScopes } from "./FindScopes";
import { FindSection } from "./FindSection";
import { FindTag } from "./FindTag";
import { FindThing } from "./FindThing";
import { FindTitle } from "./FindTitle";
import { FindWindow } from "./FindWindow";
import { files, hits, missions, places, pulls, type SampleRow } from "./samples";
import { titleParts } from "./title";

// Find, put together from its parts, as it stands at a few moments:
// finding (the pointer moves the cursor, and the pane says what it's on),
// narrowed to what the files say, in the middle of doing something to a
// thing, and with nothing found. Nothing's looked for here: what's typed
// changes nothing, and a row picked only closes it. The window's width
// says whether there's a pane, as in ws: these give it one.

const meta: Meta = { title: "Find/Find" };
export default meta;

const scopes = [
  { id: "all", label: "Everything" },
  { id: "pulls", label: "Pull requests", prefix: "#" },
  { id: "files", label: "Files", prefix: "/" },
  { id: "code", label: "Code", prefix: "%" },
  { id: "do", label: "Do", prefix: ">" },
];

// Opened, with a way to open it again once it's closed.
function Opened({ children }: { children: (close: () => void) => ReactNode }) {
  const [open, setOpen] = useState(true);
  return (
    <div className="page">
      <Button onClick={() => setOpen(true)}>Open Find</Button>
      {open && children(() => setOpen(false))}
    </div>
  );
}

const box = (value: string, placeholder: string) => <Input autoFocus value={value} placeholder={placeholder} aria-label="Find" onChange={() => {}} spellCheck={false} />;

const cardOf = (r: SampleRow) => (r.pull ? <FindPullCard pull={r.pull} /> : <FindItemCard mark={r.mark} title={r.title} kind={r.kind} hint={r.hint} />);

function Finding() {
  const groups: { title: string; rows: SampleRow[]; more?: number; says?: boolean }[] = [
    { title: "Best", rows: missions.slice(0, 1), says: true },
    { title: "Missions", rows: missions.slice(1), more: 4 },
    { title: "Pull requests", rows: pulls },
    { title: "Files", rows: files, more: 12 },
    { title: "Do", rows: places },
  ];
  const all = groups.flatMap((g) => g.rows);
  const [at, setAt] = useState(all[0]!.id);
  const [scope, setScope] = useState("all");
  const on = all.find((r) => r.id === at);
  return (
    <Opened>
      {(close) => (
        <FindWindow onClose={close} pane={<FindPane>{on ? cardOf(on) : <HoverCardLine dim>What the cursor's on shows here.</HoverCardLine>}</FindPane>}>
          <FindBar busy>{box("inv", "Find anything: a mission, a file, a pull request, an address…")}</FindBar>
          <FindScopes scopes={scopes} value={scope} onPick={setScope} />
          <FindList id="story-find">
            {groups.map((g) => (
              <FindSection key={g.title} label={g.title} title={g.title}>
                {g.rows.map((r) => (
                  <FindRow
                    key={r.id}
                    id={`story-${r.id}`}
                    on={r.id === at}
                    mark={r.mark}
                    title={<FindTitle parts={titleParts(r.title, r.at ?? [], r.path)} path={r.path} />}
                    hint={r.hint}
                    notes={r.notes}
                    tone={r.tone}
                    kind={g.says ? r.kind : undefined}
                    onOver={() => setAt(r.id)}
                    onPick={close}
                  />
                ))}
                {g.more && <FindMore count={g.more} narrows />}
              </FindSection>
            ))}
          </FindList>
          <FindFoot aside="Pull requests as of 09:12">
            <FindFootKey k="⏎">go</FindFootKey>
            <FindFootKey k="mod+⏎">new tab</FindFootKey>
            <FindFootKey k="⇥">only these</FindFootKey>
            <FindFootKey k="mod+K">act on it</FindFootKey>
            <FindFootKey k="mod+C">copy its name</FindFootKey>
          </FindFoot>
        </FindWindow>
      )}
    </Opened>
  );
}

function Code() {
  const [at, setAt] = useState(0);
  const [tagged, setTagged] = useState(true);
  const on = hits[at]!;
  return (
    <Opened>
      {(close) => (
        <FindWindow
          onClose={close}
          pane={
            <FindPane>
              <FindCodeCard path={on.path} line={on.line} repo={on.repo} worktree={on.worktree} />
            </FindPane>
          }
        >
          <FindBar
            tags={
              tagged && (
                <FindTag onClick={() => setTagged(false)} title="Search the whole workspace (⌫)">
                  repo: api
                </FindTag>
              )
            }
          >
            {box("%invoice", "Text in api's files…")}
          </FindBar>
          <FindScopes scopes={scopes} value="code" onPick={() => {}} />
          <FindList id="story-find">
            <FindSection label="Code">
              {hits.map((h, i) => (
                <FindCodeRow key={i} id={`story-${i}`} on={i === at} h={h} onOver={() => setAt(i)} onPick={close} />
              ))}
              <FindNote capped>The first 200 lines that say it. Type more to narrow it.</FindNote>
            </FindSection>
          </FindList>
          <FindFoot>
            <FindFootKey k="⏎">go</FindFootKey>
            <FindFootKey k="mod+⏎">new tab</FindFootKey>
            <FindFootKey k="⇥">everything</FindFootKey>
          </FindFoot>
        </FindWindow>
      )}
    </Opened>
  );
}

function Acting() {
  const acts = ["Start a mission about it", "Put it back on the shelf", "Dust it"];
  const [at, setAt] = useState(0);
  return (
    <Opened>
      {(close) => (
        <FindWindow
          onClose={close}
          pane={
            <FindPane scrolls>
              <FindThing>
                <FindItemCard mark={places[1]!.mark} title="THING-2: the lantern" kind="A fixture thing" hint="On the shelf, beside the kettle. Last dusted in March." />
              </FindThing>
              <FindActs>
                <FindAct icon="play" onClick={() => {}}>
                  Start a mission
                </FindAct>
                <FindAct icon="check" onClick={() => {}}>
                  Dust it
                </FindAct>
                <FindAct icon="x" disabled onClick={() => {}}>
                  Shelve it
                </FindAct>
              </FindActs>
            </FindPane>
          }
        >
          <FindBar
            tags={
              <FindTag doing onClick={close} title="Back (esc)">
                THING-2
              </FindTag>
            }
          >
            {box("", "Do something to THING-2…")}
          </FindBar>
          <FindList id="story-find" keepFocus>
            <FindSection label="What can be done to it" title="Do to THING-2">
              {acts.map((a, i) => (
                <FindRow key={a} id={`story-${i}`} on={i === at} mark={places[0]!.mark} title={<FindTitle parts={titleParts(a, [])} />} onOver={() => setAt(i)} onPick={close} />
              ))}
              <FindNote>Type it, then ⏎ sends it.</FindNote>
            </FindSection>
          </FindList>
          <FindFoot>
            <FindFootKey k="⏎">do it</FindFootKey>
            <FindFootKey k="esc">back</FindFootKey>
          </FindFoot>
        </FindWindow>
      )}
    </Opened>
  );
}

function Nothing() {
  return (
    <Opened>
      {(close) => (
        <FindWindow onClose={close}>
          <FindBar
            tags={
              <FindTag onClick={() => {}} title="Look through everything again">
                Missions
              </FindTag>
            }
          >
            {box("zzqq", "Find in missions…")}
          </FindBar>
          <FindScopes scopes={scopes} onPick={() => {}} />
          <FindList id="story-find">
            <FindNone>Nothing matches “zzqq”.</FindNone>
          </FindList>
          <FindFoot>
            <FindFootKey k="⏎">go</FindFootKey>
            <FindFootKey k="⇥">everything</FindFootKey>
          </FindFoot>
        </FindWindow>
      )}
    </Opened>
  );
}

export const FindingWithAPane: StoryObj = { name: "Finding, with a pane", render: () => <Finding /> };
export const WhatTheFilesSay: StoryObj = { name: "What the files say", render: () => <Code /> };
export const ActingOnAThing: StoryObj = { name: "Acting on a thing", render: () => <Acting /> };
export const NothingFoundNoPane: StoryObj = { name: "Nothing found, no pane", render: () => <Nothing /> };
