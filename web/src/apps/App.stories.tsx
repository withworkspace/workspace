import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Option, Select } from "../ui/Select";
import { AppBar, AppBarLink, AppBarNeed, AppBarPointer } from "./AppBar";
import { AppCard } from "./AppCard";
import { AppState } from "./AppState";
import { AppWaiting } from "./AppWaiting";

// An app's page, from its parts: the bar (a workspace app's, with its
// needs pointed at worktrees; a worktree's, with what met each need) over
// the card, framed or waiting; and how a run stands, in each of its ways.

const meta: Meta = { title: "Apps/App" };
export default meta;

const none = () => {};
const page: React.CSSProperties = { position: "relative", display: "flex", flexDirection: "column", height: 420, background: "var(--bg)" };
const frame = `data:text/html,${encodeURIComponent("<body style='font:16px system-ui;margin:24px'><h1>API docs</h1><p>Two repos' specs, drawn.</p></body>")}`;

const acts = (
  <>
    <Button variant="ghost" square title="Start it again" aria-label="Start it again">
      <Icon name="rotate" size={15} />
    </Button>
    <Button variant="ghost" square title="Stop it" aria-label="Stop it">
      <Icon name="stop" size={15} />
    </Button>
    <AppBarLink href="http://docs.workspace.localhost" />
  </>
);

// A workspace app, up and framed, a need pointed at a capsule.
export const Up: StoryObj = {
  render: () => (
    <div style={page}>
      <AppBar
        icon="book"
        title="API docs"
        end={
          <>
            <AppState kind="up" words="Up" title="It ends in 15 minutes." />
            {acts}
          </>
        }
      >
        <AppBarPointer name="api" title="api: records's api, run in the worktree picked">
          <Select size="small" value="rate-limit" onChange={none} aria-label="What api is pointed at">
            <Option value="ground" note="nearest">
              ground
            </Option>
            <Option value="rate-limit">rate-limit</Option>
          </Select>
        </AppBarPointer>
      </AppBar>
      <AppCard>
        <iframe src={frame} title="API docs" />
      </AppCard>
    </div>
  ),
};

// A worktree's app, coming up: its bar names no app (its tab does), and
// says what met each of its needs.
export const Starting: StoryObj = {
  render: () => (
    <div style={page}>
      <AppBar end={<AppState kind="starting" words="Starting" title="It ends in 15 minutes." />}>
        <AppBarNeed name="api" from="records / rate-limit" title="api: http://api.rate-limit.records.localhost" />
        <AppBarNeed name="assets" from="ground" title="assets: /tmp/ws/assets" />
      </AppBar>
      <AppCard>
        <AppWaiting log={"> vite\n  VITE ready in 412 ms\n  ➜  Local:   http://127.0.0.1:41733/"}>
          <p>Starting API docs: it's shown here once it answers.</p>
        </AppWaiting>
      </AppCard>
    </div>
  ),
};

// Ended by itself: its last words under why.
export const Exited: StoryObj = {
  render: () => (
    <div style={page}>
      <AppBar
        icon="book"
        title="API docs"
        end={
          <>
            <AppState kind="exited" words="Exited 1" />
            <Button>
              <Icon name="play" size={14} /> Start
            </Button>
          </>
        }
      />
      <AppCard>
        <AppWaiting log={"Error: Cannot find module './spec.json'\n    at Module._resolveFilename (node:internal/modules/cjs/loader:1225:15)\n    at /work/docs/serve.js:3:14"}>
          <p>API docs ended by itself (exit status 1). Its last words are below.</p>
        </AppWaiting>
      </AppCard>
    </div>
  ),
};

// Stopped from here, and every way a run stands, side by side.
export const Stopped: StoryObj = {
  render: () => (
    <div style={page}>
      <AppBar
        icon="book"
        title="API docs"
        end={
          <>
            <AppState kind="off" words="Stopped" />
            <Button>
              <Icon name="play" size={14} /> Start
            </Button>
          </>
        }
      />
      <AppCard>
        <AppWaiting>
          <p>API docs is stopped. Start it to see it again.</p>
          <p style={{ display: "flex", gap: 16 }}>
            <AppState kind="up" words="Up" />
            <AppState kind="starting" words="Starting" />
            <AppState kind="exited" words="Exited 2" />
            <AppState kind="off" words="Not running" />
          </p>
        </AppWaiting>
      </AppCard>
    </div>
  ),
};
