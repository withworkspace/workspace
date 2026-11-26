import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Copyable } from "../ui/Copyable";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { PageHead } from "../ui/PageHead";
import { Printed } from "../ui/Printed";
import { RunCame } from "./RunCame";
import { RunEnds } from "./RunEnds";
import { RunLink } from "./RunLink";
import { RunMark } from "./RunMark";
import { RunsPageFacts } from "./RunsPageFacts";
import { RunsPageFrame } from "./RunsPageFrame";
import { RunsPageLede } from "./RunsPageLede";
import { RunsPageList } from "./RunsPageList";
import { RunsPageMore, runsPageLogClass } from "./RunsPageMore";
import { RunsPageNames } from "./RunsPageNames";
import { RunsPageNote } from "./RunsPageNote";
import { RunsPagePast, RunsPagePastRun } from "./RunsPagePast";
import { RunsPageReports } from "./RunsPageReports";
import { RunsPageRow } from "./RunsPageRow";
import { RunsPageSub } from "./RunsPageSub";
import { Text, textClass } from "../ui/Text";

// The Runs page, from its parts: the whole page (its names, what's
// running, the results, a silo, what's ended, one row open); each kind of
// row side by side; where the names stand in each state; a task's runs
// as its tabs; its reports; and what's under a row opened. What's printed
// is drawn as Printed here, as a RunOutput reads it from the server.

const meta: Meta = { title: "Runs/RunsPage" };
export default meta;

const now = new Date().toISOString();
const ago = (m: number) => new Date(Date.now() - m * 60_000).toISOString();

function Actions({ live, open, onOpen }: { live?: boolean; open?: boolean; onOpen?: () => void }) {
  return (
    <>
      <Button variant="ghost" size="small" onClick={onOpen} aria-expanded={!!open}>
        <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
      </Button>
      <Button variant="ghost" size="small">
        <Icon name={live ? "stop" : "x"} size={11} /> {live ? "Stop" : "Clear"}
      </Button>
    </>
  );
}

const log = `$ python3 -m http.server 48290
Serving HTTP on 127.0.0.1 port 48290 (http://127.0.0.1:48290/) ...
127.0.0.1 - - "GET / HTTP/1.1" 200 -
127.0.0.1 - - "GET /api/greetings HTTP/1.1" 200 -`;

function ServiceRow({ up = true, open: opened = false, needs = false }: { up?: boolean; open?: boolean; needs?: boolean }) {
  const [open, setOpen] = useState(opened);
  return (
    <RunsPageRow
      open={open}
      mark={<RunMark live up={up} title={up ? "Up" : "Not up yet"} />}
      name="web"
      nameTitle="web.a-page-that-needs-an-api.demo"
      top={
        <>
          <RunLink href="http://web.storefront.demo.sorrel.localhost" title="Open it">
            web.storefront.demo.sorrel.localhost
          </RunLink>
          {!up && <Chip tone="quiet">not up</Chip>}
        </>
      }
      sub={
        <>
          <RunsPageSub>
            <span>
              In <a href="#">a-page-that-needs-an-api</a>
            </span>
            <span>
              For <a href="#">crew c2</a>
            </span>
            <Text mono>127.0.0.1:48290</Text>
          </RunsPageSub>
          {needs && (
            <RunsPageSub>
              <span>
                Needs api:{" "}
                <a href="#" className={textClass("mono")}>
                  api.ground.records
                </a>
                , at{" "}
                <a href="#" className={textClass("mono")}>
                  /api
                </a>
              </span>
              <span>
                Needs cache: <Text mono>cache.ground.records</Text> <Chip tone="waiting">not running</Chip>
              </span>
            </RunsPageSub>
          )}
          <RunEnds>Ends when crew c2 is done, or in 3h54</RunEnds>
        </>
      }
      started={ago(6)}
      actions={<Actions live open={open} onOpen={() => setOpen(!open)} />}
      more={
        <RunsPageMore>
          <RunsPageFacts>
            <dt>Command</dt>
            <Text as="dd" mono>
              <Copyable text="python3 serve.py">python3 serve.py</Copyable>
            </Text>
            <dt>Folder</dt>
            <Text as="dd" mono>~/sorrel/repos/demo/a-page-that-needs-an-api</Text>
            <dt>Started</dt>
            <dd>
              Today at 09:12 by crew c2, from its <code>[run.service.web]</code>
            </dd>
            <dt>Process</dt>
            <dd>pid 84361, on port 48290</dd>
          </RunsPageFacts>
          <Printed className={runsPageLogClass}>{log}</Printed>
        </RunsPageMore>
      }
    />
  );
}

function TaskRow({ result, open: opened = false }: { result: "passed" | "failed" | "running" | "stopped"; open?: boolean }) {
  const [open, setOpen] = useState(opened);
  const [picked, setPicked] = useState(3);
  const came = result === "passed" ? "passed in 2.1s" : result === "failed" ? "failed (exit 1) in 0.4s" : result === "running" ? "running" : "stopped";
  return (
    <RunsPageRow
      open={open}
      mark={result === "running" ? <RunMark live up title="Running" /> : <RunMark task={result} title={came} />}
      name={result === "failed" ? "lint" : "test"}
      top={
        <>
          <RunCame result={result}>
            {came}
            {result !== "running" && <Text dim>, 4 minutes ago</Text>}
          </RunCame>
          {result === "passed" && <Text dim>run 3</Text>}
        </>
      }
      sub={
        <RunsPageSub>
          <span>
            In <a href="#">count-the-greetings</a>
          </span>
          <span>At 3f2a91c</span>
          {result === "passed" && (
            <span>
              Check 2 of step 1 of <a href="#">Count the greetings</a>
            </span>
          )}
          <RunsPageReports reports={[{ name: "coverage", title: "cover.out, go: kept with the run", unwritten: result === "failed" }]} />
        </RunsPageSub>
      }
      actions={<Actions live={result === "running"} open={open} onOpen={() => setOpen(!open)} />}
      more={
        <RunsPageMore>
          <RunsPagePast label="Runs of test">
            {[3, 2, 1].map((n) => (
              <RunsPagePastRun key={n} n={n} result={n === 2 ? "failed" : "passed"} at={ago(n * 7)} on={picked === n} title={`Run ${n}`} onPick={() => setPicked(n)} />
            ))}
          </RunsPagePast>
          <RunsPageFacts>
            <dt>Result</dt>
            <RunCame as="dd" result={result}>
              {came}
            </RunCame>
            <dt>Command</dt>
            <Text as="dd" mono>go test -coverprofile=cover.out ./...</Text>
            <dt>Commit</dt>
            <dd>
              <Text mono>3f2a91c0b7de</Text>
              <Text dim>, with changes that weren't committed</Text>
            </dd>
            <dt>Reports</dt>
            <dd>
              <RunsPageReports reports={[{ name: "coverage", title: "cover.out, go: kept with the run", unwritten: false }]} />
            </dd>
          </RunsPageFacts>
          <Printed className={runsPageLogClass}>{"ok  \tdemo/greetings\t0.21s\tcoverage: 81.2% of statements"}</Printed>
        </RunsPageMore>
      }
    />
  );
}

function SiloRow({ trouble }: { trouble?: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <RunsPageRow
      open={open}
      mark={<RunMark live up title="Kept as files change" />}
      name="demo"
      top={
        <>
          <Text dim>on</Text>
          <Text mono>silo-a</Text>
          {trouble && <Chip tone="waiting">sync failed</Chip>}
        </>
      }
      sub={
        <RunsPageSub>
          <span>Watching 4 folders</span>
          <span>{trouble ? "Last sync failed" : "Last synced 3 files"}, 2 minutes ago</span>
          <span>post-sync ran 2 minutes ago</span>
        </RunsPageSub>
      }
      actions={
        <>
          <Button variant="ghost" size="small" onClick={() => setOpen(!open)} aria-expanded={open}>
            <Icon name={open ? "chevron-up" : "terminal"} size={11} /> {open ? "Hide" : "Output"}
          </Button>
          <Button variant="ghost" size="small">
            <Icon name="rotate" size={11} /> Resync
          </Button>
        </>
      }
      more={
        <RunsPageMore>
          {trouble && <p>rsync: write failed: no space left on device</p>}
          <Text as="p" dim>Its post-sync hook</Text>
          <Printed className={runsPageLogClass}>{"go build ./... \nok"}</Printed>
        </RunsPageMore>
      }
    />
  );
}

function EndedRow() {
  return (
    <RunsPageRow
      ended
      mark={<RunMark title="Ended" />}
      name="sh"
      top={<Text dim>exited 3, 2 minutes ago</Text>}
      sub={
        <RunsPageSub>
          <span>
            In <a href="#">a-page-that-needs-an-api</a>
          </span>
          <span>For you</span>
        </RunsPageSub>
      }
      started={ago(3)}
      actions={<Actions />}
    />
  );
}

function Names({ note, router = true }: { note?: boolean; router?: boolean }) {
  return (
    <RunsPageNames
      names={
        <>
          <a href="#" className={textClass("mono")}>
            sorrel.localhost
            <Icon name="arrow-up-right" size={11} />
          </a>
          <Text dim> and each run below it, as web.&lt;capsule&gt;.&lt;repo&gt;.sorrel.localhost</Text>
        </>
      }
      note={note ? <>sorrel was another workspace's first: this one is sorrel-2 while that one's daemon runs.</> : undefined}
      router={
        router ? (
          <>
            On :80<Text dim>. One serves every workspace on this machine.</Text>
          </>
        ) : (
          <Text dim>None running</Text>
        )
      }
    />
  );
}

export const Page: StoryObj = {
  render: () => (
    <RunsPageFrame>
      <PageHead title="Runs">
        <Chip tone="done">2 running</Chip>
      </PageHead>
      <RunsPageLede>
        What ws runs for you and your agents. A service (a dev server, a preview, a compose stack) is given a free port, and ended once whoever it's for goes, or its time's up. A
        task (tests, lint, a build) ends by itself, and what it came to is kept.
      </RunsPageLede>
      <Names />
      <section>
        <GroupHead count={2}>Running</GroupHead>
        <RunsPageList>
          <ServiceRow needs open />
          <ServiceRow up={false} />
        </RunsPageList>
      </section>
      <section>
        <GroupHead count={2}>Results</GroupHead>
        <RunsPageNote>What each task last came to, kept with all it printed, its reports, and the runs of it before, while where it ran is there.</RunsPageNote>
        <RunsPageList>
          <TaskRow result="passed" />
          <TaskRow result="failed" />
        </RunsPageList>
      </section>
      <section>
        <GroupHead count={1}>Silos</GroupHead>
        <RunsPageNote>
          Each mirrors the capsule it's pointed at, kept by the daemon as files change. <code>ws silo point</code> points one elsewhere.
        </RunsPageNote>
        <RunsPageList>
          <SiloRow />
        </RunsPageList>
      </section>
      <section>
        <GroupHead count={1}>Ended</GroupHead>
        <RunsPageNote>Services that ended by themselves, kept an hour with their output.</RunsPageNote>
        <RunsPageList>
          <EndedRow />
        </RunsPageList>
      </section>
    </RunsPageFrame>
  ),
};

// Each kind of row: a service up and not up yet, one with what it needs;
// a task passed, failed, running, stopped, one open with the runs before
// it; a silo, and one whose sync failed, opened; a service that ended.
export const Rows: StoryObj = {
  render: () => (
    <RunsPageFrame>
      <RunsPageList>
        <ServiceRow />
        <ServiceRow up={false} />
        <ServiceRow needs />
        <TaskRow result="passed" />
        <TaskRow result="failed" />
        <TaskRow result="running" />
        <TaskRow result="stopped" />
        <TaskRow result="passed" open />
        <SiloRow />
        <SiloRow trouble />
        <EndedRow />
      </RunsPageList>
    </RunsPageFrame>
  ),
};

// Where the names stand: served, another workspace's name taken, and no
// router yet.
export const NamesStates: StoryObj = {
  name: "Names",
  render: () => (
    <RunsPageFrame>
      <Names />
      <Names note />
      <Names router={false} />
    </RunsPageFrame>
  ),
};

// A task's runs as tabs: passed, failed, running and stopped; the second
// is the one open. Its reports: kept, and one its command didn't write.
export const PastAndReports: StoryObj = {
  name: "Past and reports",
  render: () => (
    <RunsPageFrame>
      <RunsPageMore>
        <RunsPagePast label="Runs of test">
          <RunsPagePastRun n={4} result="running" at={now} on={false} title="Run 4: running" onPick={() => {}} />
          <RunsPagePastRun n={3} result="passed" at={ago(5)} on title="Run 3: passed" onPick={() => {}} />
          <RunsPagePastRun n={2} result="failed" at={ago(30)} on={false} title="Run 2: failed" onPick={() => {}} />
          <RunsPagePastRun n={1} result="stopped" at={ago(90)} on={false} title="Run 1: stopped" onPick={() => {}} />
        </RunsPagePast>
        <RunsPageReports
          reports={[
            { name: "coverage", title: "cover.out, go: kept with the run", unwritten: false },
            { name: "junit", title: "Its command didn't write report.xml", unwritten: true },
          ]}
        />
      </RunsPageMore>
    </RunsPageFrame>
  ),
};
