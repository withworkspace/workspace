import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { RunCame } from "../../runs/RunCame";
import { RunEnds } from "../../runs/RunEnds";
import { RunLink } from "../../runs/RunLink";
import { RunMark } from "../../runs/RunMark";
import { RunName } from "../../runs/RunName";
import { Button } from "../../ui/Button";
import { Chip } from "../../ui/Chip";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Status } from "../../ui/Status";
import { MissionEndingCapsules } from "../Ending/MissionEndingCapsules";
import { MissionCrewBadge } from "../MissionCrewBadge";
import { MissionOutputState } from "../MissionOutputState";
import { MissionOutputThreads } from "../MissionOutputThreads";
import { MissionWork } from "./MissionWork";
import { MissionWorkBody } from "./MissionWorkBody";
import { MissionWorkCommits } from "./MissionWorkCommits";
import { MissionWorkCrew } from "./MissionWorkCrew";
import { MissionWorkCrewLine } from "./MissionWorkCrewLine";
import { MissionWorkEmpty } from "./MissionWorkEmpty";
import { MissionWorkEnded } from "./MissionWorkEnded";
import { MissionWorkFile } from "./MissionWorkFile";
import { MissionWorkFileGroup } from "./MissionWorkFileGroup";
import { MissionWorkFilter } from "./MissionWorkFilter";
import { MissionWorkGroup } from "./MissionWorkGroup";
import { MissionWorkHead } from "./MissionWorkHead";
import { MissionWorkHeading } from "./MissionWorkHeading";
import { MissionWorkLeft } from "./MissionWorkLeft";
import { MissionWorkLines } from "./MissionWorkLines";
import { MissionWorkLoose } from "./MissionWorkLoose";
import { MissionWorkMade } from "./MissionWorkMade";
import { MissionWorkNone } from "./MissionWorkNone";
import { MissionWorkNow } from "./MissionWorkNow";
import { MissionWorkProse } from "./MissionWorkProse";
import { MissionWorkRecipes } from "./MissionWorkRecipes";
import { MissionWorkRow } from "./MissionWorkRow";
import { MissionWorkRun } from "./MissionWorkRun";
import { MissionWorkRuns } from "./MissionWorkRuns";
import { MissionWorkState } from "./MissionWorkState";
import { MissionWorkStep } from "./MissionWorkStep";
import { MissionWorkStepBundle } from "./MissionWorkStepBundle";
import { MissionWorkSteps } from "./MissionWorkSteps";
import { MissionWorkSub } from "./MissionWorkSub";
import { Text } from "../../ui/Text";

// A mission's Work tab, from its parts: the whole tab, with a capsule open
// (its head, the step the mission's on, who's been in it, its runs, steps
// and commits, what was made there, and its changed files beside); an
// output open, as prose and as it's written; the list's rows in each
// state; and what's said where there's nothing.

const meta: Meta = { title: "Mission/MissionWork" };
export default meta;

const none = () => {};
const at = "2026-11-24T10:00:00Z";
const box = <Icon name="box" size={14} />;

function List({ folded }: { folded?: boolean }) {
  const [open, setOpen] = useState(!folded);
  return (
    <>
      <MissionWorkGroup count={4}>Capsules</MissionWorkGroup>
      <MissionWorkSub>In hand</MissionWorkSub>
      <MissionWorkRow
        icon={box}
        name="API"
        stat={<MissionWorkLines additions={128} deletions={14} />}
        end={<MissionWorkState text="c2 working" tone="progress" live />}
        selected
        title="api/rate-limit-the-public-api"
        onSelect={none}
      />
      <MissionWorkRow
        icon={box}
        name="Docs"
        slug="limits-page"
        stat="2 not committed"
        end={
          <>
            <MissionCrewBadge id="c3" live />
            <MissionWorkState text="Yours to check" tone="attention" icon="eye" />
          </>
        }
        onSelect={none}
      />
      <MissionWorkSub>Up next</MissionWorkSub>
      <MissionWorkRow icon={box} name="Gateway" slug="later" end={<MissionWorkState text="From step 6" />} quiet onSelect={none} />
      <MissionWorkSub open={open} onToggle={() => setOpen(!open)} rest="Records">
        Done
      </MissionWorkSub>
      {open && <MissionWorkRow icon={box} name="Records" stat={<MissionWorkLines additions={31} deletions={0} />} end={<MissionWorkState text="Landed Nov 23" tone="done" icon="check" />} quiet onSelect={none} />}
      <MissionWorkEnded>
        <MissionEndingCapsules capsules={[{ repo: "api", name: "spike", why: "the spike answered it", dropped: true }]} />
      </MissionWorkEnded>
      <MissionWorkGroup count={4}>Outputs</MissionWorkGroup>
      <MissionWorkRow icon={<Icon name="file" size={14} />} name="Why the limit is a key's, not an address's" end={<MissionOutputState state="landed" />} output onSelect={none} />
      <MissionWorkRow
        icon={<Icon name="file" size={14} />}
        name="Open questions"
        end={<MissionOutputThreads t={{ open: 2, waiting: 1 }} />}
        when={at}
        output
        onSelect={none}
      />
      <MissionWorkRow icon={<MissionCrewBadge id="c2" />} name="The limiter is in the gateway, with a test for each header it sends." when={at} output plain onSelect={none} />
      <MissionWorkRow icon={<MissionCrewBadge id="c1" />} name="Design written as a plan: where the count is kept, and what a client is told." when={at} output plain onSelect={none} />
    </>
  );
}

function Files() {
  const [filter, setFilter] = useState("");
  const [open, setOpen] = useState(true);
  return (
    <>
      <MissionWorkHeading count={9} end={<MissionWorkFilter value={filter} onChange={setFilter} />}>
        Changed files
      </MissionWorkHeading>
      <MissionWorkFileGroup dir="gateway/" n={5} additions={120} deletions={14} open={open} onToggle={() => setOpen(!open)}>
        <MissionWorkFile to="/" status="added" dir="limit/" name="limit.go" additions={74} deletions={0} title="gateway/limit/limit.go" />
        <MissionWorkFile to="/" status="added" dir="limit/" name="limit_test.go" additions={38} deletions={0} loose title="gateway/limit/limit_test.go · not committed" />
        <MissionWorkFile to="/" status="modified" dir="" name="server.go" additions={8} deletions={2} title="gateway/server.go" />
        <MissionWorkFile status="deleted" dir="old/" name="throttle.go" additions={0} deletions={12} title="gateway/old/throttle.go" />
        <MissionWorkFile to="/" status="renamed" dir="a/folder/that/is/long/enough/to/be/cut/short/from/the/left/" name="headers.go" additions={0} deletions={0} title="gateway/headers.go" />
      </MissionWorkFileGroup>
      <MissionWorkFileGroup dir="top level" n={2} additions={8} deletions={0} open onToggle={none}>
        <MissionWorkFile to="/" status="modified" dir="" name="go.mod" additions={2} deletions={0} title="go.mod" />
        <MissionWorkFile to="/" status="modified" dir="" name="go.sum" additions={6} deletions={0} skim title="go.sum · one to skim: a lock file" />
      </MissionWorkFileGroup>
      <Text as="p" dim>and 2 more</Text>
    </>
  );
}

function Capsule() {
  return (
    <>
      <MissionWorkHead
        icon="box"
        name="API"
        slug="api/rate-limit-the-public-api"
        tags={
          <>
            <Chip tone="quiet" title="Its branch">
              <Icon name="branch" size={11} />
              rate-limit
            </Chip>
            <MissionWorkState text="c2 working" tone="progress" live />
          </>
        }
        actions={
          <>
            <Button size="small">
              <Icon name="file" size={12} /> Files <Keys k="f" />
            </Button>
            <Button size="small">
              <Icon name="eye" /> Review <Keys k="v" />
            </Button>
          </>
        }
        meta={
          <>
            <span>3 commits since it left main</span>
            <span>
              9 files <MissionWorkLines additions={128} deletions={14} />
            </span>
            <MissionWorkLoose loose>1 file not committed</MissionWorkLoose>
            <span>2 behind</span>
          </>
        }
        pr="Answer 429 over the limit, with when to try again"
      />
      <MissionWorkBody files={<Files />}>
        <MissionWorkNow
          tone="progress"
          actions={
            <Button size="small">
              <Icon name="terminal" size={12} /> Watch c2
            </Button>
          }
        >
          <strong>Step 3 · Limit each key</strong> has c2 on it, since 09:41.
        </MissionWorkNow>
        <MissionWorkHeading count={2}>Crew here</MissionWorkHeading>
        <MissionWorkCrew>
          <MissionWorkCrewLine
            badge={<MissionCrewBadge id="c2" live />}
            to="/"
            link={
              <>
                <Icon name="terminal" size={12} /> Terminal
              </>
            }
          >
            <Status s={{ icon: "circle-dot", tone: "progress", words: "Working", live: true }} /> <Text dim>step 3, Limit each key · today 09:41, still here</Text>
          </MissionWorkCrewLine>
          <MissionWorkCrewLine badge={<MissionCrewBadge id="c1" />} to="/" link="Transcript" left={<MissionWorkLeft onOpen={none}>Design written as a plan: where the count is kept, and what a client is told.</MissionWorkLeft>}>
            <strong>Done</strong> <Text dim>step 1, Design it · yesterday 16:02 to 16:40</Text>
          </MissionWorkCrewLine>
          <MissionWorkCrewLine badge={<MissionCrewBadge id="c4" />} to="/" link="Transcript" left={<MissionWorkLeft>Stopped before it wrote anything: the gateway's tests were already failing on main.</MissionWorkLeft>}>
            <strong>Stopped</strong> <Text dim>step 2, Count requests · yesterday 16:44 to 16:51</Text>
          </MissionWorkCrewLine>
        </MissionWorkCrew>
        <MissionWorkHeading count={5}>Runs</MissionWorkHeading>
        <MissionWorkRuns>
          <MissionWorkRun
            mark={<RunMark live up />}
            under={
              <>
                <RunEnds>
                  Needs redis (<Text mono>cache / main</Text>)
                </RunEnds>
                <RunEnds>Ends 15 minutes after you close the mission</RunEnds>
              </>
            }
            action={
              <Button variant="ghost" size="small">
                <Icon name="stop" size={11} /> Stop
              </Button>
            }
          >
            <RunName>gateway</RunName>
            <RunLink href="http://gateway.api.localhost">gateway.api.localhost</RunLink>
          </MissionWorkRun>
          <MissionWorkRun
            mark={<RunMark live />}
            action={
              <Button variant="ghost" size="small">
                <Icon name="stop" size={11} /> Stop
              </Button>
            }
          >
            <RunName>docs</RunName>
            <RunLink href="http://127.0.0.1:4100">127.0.0.1:4100</RunLink>
            <Chip tone="quiet">not up</Chip>
          </MissionWorkRun>
          <MissionWorkRun mark={<RunMark ended />}>
            <RunName>worker</RunName>
            <Text dim>exited 0, 4 minutes ago</Text>
          </MissionWorkRun>
          <MissionWorkRun mark={<RunMark task="passed" />} under={<RunEnds>At 3f9c2ab · check 1 of step 3</RunEnds>}>
            <RunName to="/">test</RunName>
            <RunCame result="passed">
              Passed in 12s<Text dim>, 2 minutes ago</Text>
            </RunCame>
          </MissionWorkRun>
          <MissionWorkRun mark={<RunMark task="failed" />} under={<RunEnds>At 3f9c2ab</RunEnds>}>
            <RunName to="/">lint</RunName>
            <RunCame result="failed">
              Failed (exit 1) in 4s<Text dim>, 2 minutes ago</Text>
            </RunCame>
          </MissionWorkRun>
        </MissionWorkRuns>
        <MissionWorkRecipes>
          <Button size="small">
            <Icon name="play" size={11} /> Run bench
          </Button>
        </MissionWorkRecipes>
        <MissionWorkHeading count={3}>Steps here</MissionWorkHeading>
        <MissionWorkSteps>
          <MissionWorkStep n={2} title="Count requests a key" state="accepted" hint="Count requests a key: its checks were met">
            <MissionCrewBadge id="c4" />
            <Text dim>Work</Text>
            <MissionWorkStepBundle id="b2" to="/" title="b2: approved" />
          </MissionWorkStep>
          <MissionWorkStep n={3} title="Limit each key" state="current">
            <MissionCrewBadge id="c2" live />
            <Text dim>Work</Text>
            <Chip tone="info">now</Chip>
          </MissionWorkStep>
          <MissionWorkStep n={4} title="Read it" state="todo">
            <Chip tone="quiet">you</Chip>
            <Text dim>Review</Text>
          </MissionWorkStep>
        </MissionWorkSteps>
        <MissionWorkHeading count={5}>Commits</MissionWorkHeading>
        <MissionWorkCommits
          commits={[
            { sha: "3f9c2ab41d0e", subject: "feat(gateway): answer 429 over the limit, with Retry-After", time: at },
            { sha: "a81b07c9e2f4", subject: "feat(limit): a limit a key, counted in Redis, with a window that slides rather than one that resets on the minute", time: at },
            { sha: "0c4de5519a3b", subject: "test(limit): the window's edges", time: at },
          ]}
          more={2}
        />
        <MissionWorkHeading count={1}>Made here</MissionWorkHeading>
        <MissionWorkMade>
          <MissionWorkRow icon={<Icon name="file" size={14} />} name="Why the limit is a key's, not an address's" end={<MissionOutputState state="landed" />} output onSelect={none} />
        </MissionWorkMade>
      </MissionWorkBody>
    </>
  );
}

// The tab as the mission page has it when it doesn't fill the window: the
// list, and the open capsule in its card.
export const Tab: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 1400 }}>
      <MissionWork list={<List />}>
        <Capsule />
      </MissionWork>
    </div>
  ),
};

// Done folded to a line, and nothing open: there's no card.
export const Folded: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 1400 }}>
      <MissionWork list={<List folded />} />
    </div>
  ),
};

function Output({ code }: { code?: boolean }) {
  return (
    <>
      <MissionWorkHead
        kind={<>Handoff · step 1, Design it · Nov 23</>}
        actions={
          <Button size="small">
            <Icon name="arrow-up-right" /> Open its page <Keys k="⏎" />
          </Button>
        }
        title="Design written as a plan: where the count is kept, and what a client is told."
        meta={
          <>
            <span>
              <MissionCrewBadge id="c1" /> left it
            </span>
            <MissionOutputState state="private" />
            <MissionOutputThreads t={{ open: 2, waiting: 1 }} />
          </>
        }
      />
      <MissionWorkBody>
        {code ? (
          <MissionWorkProse code>{`limit:\n  per: key\n  window: 60s\n  requests: 120\n`}</MissionWorkProse>
        ) : (
          <MissionWorkProse>
            <p>The count is kept in Redis, a key for each API key, with a window that slides. Over the limit the gateway answers 429 with Retry-After, in seconds.</p>
            <p>
              What's left open: whether a key's limit can be raised for a customer, and where that's set. See <a href="#open">Open questions</a>.
            </p>
          </MissionWorkProse>
        )}
      </MissionWorkBody>
    </>
  );
}

// An output open: prose, then one that's shown as it's written.
export const Outputs: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 24, maxWidth: 900 }}>
      <MissionWork list={null}>
        <Output />
      </MissionWork>
      <MissionWork list={null}>
        <Output code />
      </MissionWork>
    </div>
  ),
};

// Where a capsule can stand, in its chip, and a row in each state.
export const Rows: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 16, width: 400 }}>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
        <MissionWorkState text="c2 working" tone="progress" live />
        <MissionWorkState text="c3 asks" tone="attention" />
        <MissionWorkState text="Yours to check" tone="attention" icon="eye" />
        <MissionWorkState text="api#46 open" tone="waiting" icon="pr" url="https://github.com/acme/api/pull/46" />
        <MissionWorkState text="Landed Nov 23" tone="done" icon="check" />
        <MissionWorkState text="From step 6" />
      </div>
      <div>
        <MissionWorkRow icon={box} name="API" stat={<MissionWorkLines additions={128} deletions={14} />} end={<MissionWorkState text="c2 working" tone="progress" live />} onSelect={none} />
        <MissionWorkRow icon={box} name="API" stat={<MissionWorkLines additions={128} deletions={14} />} end={<MissionWorkState text="c2 working" tone="progress" live />} selected onSelect={none} />
        <MissionWorkRow icon={box} name="A repo with a long name" slug="and-a-capsule-whose-own-name-is-longer-still" stat="12 not committed" end={<MissionWorkState text="Yours to check" tone="attention" />} onSelect={none} />
        <MissionWorkRow icon={box} name="Gateway" slug="later" end={<MissionWorkState text="From step 6" />} quiet onSelect={none} />
        <MissionWorkRow icon={<Icon name="file" size={14} />} name="Open questions" when={at} output onSelect={none} />
        <MissionWorkRow icon={<MissionCrewBadge id="c2" />} name="The limiter is in the gateway, with a test for each header it sends." when={at} output plain onSelect={none} />
      </div>
    </div>
  ),
};

// Nothing made yet, and a capsule nothing's changed in.
export const Nothing: StoryObj = {
  render: () => (
    <div className="page" style={{ maxWidth: 1100 }}>
      <MissionWork
        list={
          <>
            <MissionWorkGroup count={1}>Capsules</MissionWorkGroup>
            <MissionWorkSub>Up next</MissionWorkSub>
            <MissionWorkRow icon={box} name="Gateway" end={<MissionWorkState text="Not started" />} quiet selected onSelect={none} />
            <MissionWorkGroup count={0}>Outputs</MissionWorkGroup>
            <MissionWorkNone>Nothing yet: proposals, notes, handoffs and artifacts show here as crew make them.</MissionWorkNone>
          </>
        }
      >
        <MissionWorkHead icon="box" name="Gateway" slug="gateway/rate-limit-the-public-api" tags={<MissionWorkState text="Not started" />} meta={<span>No commits yet</span>} />
        <MissionWorkBody>
          <MissionWorkEmpty icon="box" title="Nothing's changed here yet">
            It's level with main at 3f9c2ab. Step 1, Design it, is the first in it.
          </MissionWorkEmpty>
        </MissionWorkBody>
      </MissionWork>
    </div>
  ),
};
