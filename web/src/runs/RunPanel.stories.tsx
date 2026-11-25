import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Printed } from "../ui/Printed";
import { RunPanelBody, runPanelLogClass } from "./RunPanelBody";
import { RunPanelAside, RunPanelCommand, RunPanelFact, RunPanelFacts, RunPanelReports } from "./RunPanelFacts";
import { RunPanelFrame } from "./RunPanelFrame";
import { RunPanelHead } from "./RunPanelHead";
import { RunPanelIdle } from "./RunPanelIdle";
import { RunPanelNeed, RunPanelNeeds } from "./RunPanelNeeds";
import { RunPanelViews } from "./RunPanelViews";
import type { RunView } from "./runView";

// One run as a worktree's Runs tab shows the one picked, from its parts:
// a service that's up (its address, what it needs, Preview and Output),
// one starting; a task that passed and one that failed (which run it is,
// its reports, its output alone); and a service that isn't running, with
// Start. What's printed is drawn as Printed here, as a RunOutput reads
// it from the server; the frame is a stand-in.

const meta: Meta = { title: "Runs/RunPanel" };
export default meta;

// The panel fills a column of the Files page, as tall as the window.
function Column({ children }: { children: ReactNode }) {
  return <div style={{ display: "flex", height: 620, border: "1px dashed var(--line)" }}>{children}</div>;
}

const log = `Serving HTTP on 127.0.0.1 port 48290 (http://127.0.0.1:48290/) ...
127.0.0.1 - - "GET / HTTP/1.1" 200 -
127.0.0.1 - - "GET /api/greetings HTTP/1.1" 200 -`;

function Service({ up = true }: { up?: boolean }) {
  const [shown, setShown] = useState<RunView>("preview");
  return (
    <Column>
      <RunPanelFrame label={`Storefront, ${up ? "up" : "starting"}`}>
        <RunPanelHead
          title="Storefront"
          name="web"
          status={{ icon: up ? "circle-dot" : "circle-dashed", tone: up ? "done" : "waiting", words: up ? "Up on :48290" : "Starting on :48290" }}
          address={{ href: "#", host: "web.a-page-that-needs-an-api.demo.sorrel.localhost", title: "Open it in a tab of its own: also 127.0.0.1:48290" }}
        >
          <Button size="small">
            <Icon name="play" size={11} /> Run again
          </Button>
          <Button variant="ghost" size="small">
            <Icon name="stop" size={11} /> Stop
          </Button>
        </RunPanelHead>
        <RunPanelFacts>
          <RunPanelFact label="Started">
            6 minutes ago
            <RunPanelAside>by you, from this tab</RunPanelAside>
          </RunPanelFact>
          <RunPanelFact label="For">
            <a href="#">crew c2</a>
            <RunPanelAside>so it ends when crew c2 is done</RunPanelAside>
          </RunPanelFact>
          <RunPanelFact label="Recipe">
            <span className="mono">[run.service.web]</span>
            <RunPanelAside>An app, framed here</RunPanelAside>
          </RunPanelFact>
          <RunPanelFact label="Needs" wide>
            <RunPanelNeeds>
              <RunPanelNeed name="api">
                <a href="#" className="mono">
                  api.ground.records
                </a>
                <RunPanelAside>as $API_URL</RunPanelAside>
                <RunPanelAside>at /api</RunPanelAside>
              </RunPanelNeed>
              <RunPanelNeed name="docs">
                <a href="#" className="mono">
                  records / ground
                </a>
              </RunPanelNeed>
            </RunPanelNeeds>
          </RunPanelFact>
          <RunPanelFact label="Needed by">admin</RunPanelFact>
          <RunPanelFact label="Command" wide>
            <RunPanelCommand command="python3 serve.py --port $PORT" />
          </RunPanelFact>
        </RunPanelFacts>
        <RunPanelViews framed shown={shown} onChange={setShown} controls="story-web" name="web" />
        <RunPanelBody id="story-web" tabpanel label={shown === "preview" ? "web, framed" : "web's output"}>
          {shown === "preview" ? (
            <div style={{ flex: 1, display: "grid", placeItems: "center", background: "var(--panel)", borderRadius: 8 }}>A stand-in for the frame</div>
          ) : (
            <Printed className={runPanelLogClass}>{log}</Printed>
          )}
        </RunPanelBody>
      </RunPanelFrame>
    </Column>
  );
}

function Task({ passed = true }: { passed?: boolean }) {
  return (
    <Column>
      <RunPanelFrame label={`test, ${passed ? "passed" : "failed"}`}>
        <RunPanelHead
          name={passed ? "test" : "lint"}
          n={3}
          status={{ icon: passed ? "check-circle" : "x-circle", tone: passed ? "done" : "danger", words: passed ? "Passed in 2.1s" : "Failed (exit 1) in 0.4s" }}
        >
          <Button size="small">
            <Icon name="play" size={11} /> Run again
          </Button>
          <Button variant="ghost" size="small">
            <Icon name="x" size={11} /> Clear
          </Button>
        </RunPanelHead>
        <RunPanelFacts>
          <RunPanelFact label="Started">
            4 minutes ago
            <RunPanelAside>by ws, for a check</RunPanelAside>
          </RunPanelFact>
          <RunPanelFact label="For">
            <a href="#">check 2 of step 1 of Count the greetings</a>
          </RunPanelFact>
          <RunPanelFact label="Ended">4 minutes ago</RunPanelFact>
          <RunPanelFact label="Commit">
            <span className="mono">3f2a91c0b</span>
            <RunPanelAside>with changes not committed</RunPanelAside>
          </RunPanelFact>
          <RunPanelFact label="Recipe">
            <span className="mono">[run.task.test]</span>
          </RunPanelFact>
          <RunPanelFact label="Reports">
            <RunPanelReports>
              <Chip tone={passed ? "quiet" : "waiting"} title="cover.out, go">
                <Icon name="file" size={10} /> coverage{passed ? "" : " not written"}
              </Chip>
            </RunPanelReports>
          </RunPanelFact>
          <RunPanelFact label="Command" wide>
            <RunPanelCommand command="go test -coverprofile=cover.out ./..." />
          </RunPanelFact>
        </RunPanelFacts>
        <RunPanelViews framed={false} shown="output" onChange={() => {}} controls="story-test" name="test" />
        <RunPanelBody id="story-test" tabpanel={false} label="test's output">
          <Printed className={runPanelLogClass}>{passed ? "ok  \tdemo/greetings\t0.21s\tcoverage: 81.2% of statements" : "greetings.go\nexit status 1"}</Printed>
        </RunPanelBody>
      </RunPanelFrame>
    </Column>
  );
}

function Recipe({ inTab }: { inTab?: boolean }) {
  return (
    <Column>
      <RunPanelFrame label="Files, not running">
        <RunPanelHead title={inTab ? "Files" : undefined} name="serve" status={{ icon: "circle", tone: "quiet", words: "Not running" }}>
          <Button variant="primary" size="small">
            <Icon name="play" size={11} /> Start
          </Button>
        </RunPanelHead>
        <RunPanelFacts>
          <RunPanelFact label="Recipe">
            <span className="mono">[run.service.serve]</span>
            <RunPanelAside>{inTab ? "A tool, opened in a tab of its own" : "An app, framed here"}</RunPanelAside>
          </RunPanelFact>
          <RunPanelFact label="Command" wide>
            <RunPanelCommand command="python3 -m http.server $PORT" />
          </RunPanelFact>
        </RunPanelFacts>
        <RunPanelIdle icon={inTab ? "external" : "play"} title={`${inTab ? "Files" : "serve"} isn't running`} action={inTab ? <a href="#">Its tab, Files</a> : undefined}>
          {inTab
            ? "Start it, and it opens in a tab of its own once it answers. It runs while you have this open, and 15 minutes after."
            : "Start it, and it's shown here, in a frame, once it answers. It runs while you have this open, and 15 minutes after."}
        </RunPanelIdle>
      </RunPanelFrame>
    </Column>
  );
}

export const ServiceUp: StoryObj = { name: "Service, up", render: () => <Service /> };
export const ServiceStarting: StoryObj = { name: "Service, starting", render: () => <Service up={false} /> };
export const TaskPassed: StoryObj = { name: "Task, passed", render: () => <Task /> };
export const TaskFailed: StoryObj = { name: "Task, failed", render: () => <Task passed={false} /> };
export const NotRunning: StoryObj = { name: "Not running", render: () => <Recipe /> };
export const NotRunningInTab: StoryObj = { name: "Not running, in a tab of its own", render: () => <Recipe inTab /> };
