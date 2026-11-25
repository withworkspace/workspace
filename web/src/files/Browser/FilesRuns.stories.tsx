import type { Meta, StoryObj } from "@storybook/react-vite";
import { Link } from "react-router";
import { Button } from "../../ui/Button";
import { Empty } from "../../ui/Empty";
import { Icon } from "../../ui/Icon";
import { Row, RowMain, RowTitle, rowLinkClass } from "../../ui/Rows";
import { Spinner } from "../../ui/Spinner";
import { FilesRuns } from "./FilesRuns";
import { FilesRunsBack } from "./FilesRunsBack";
import { FilesRunsGroup } from "./FilesRunsGroup";
import { FilesRunsNone } from "./FilesRunsNone";

// A worktree's Runs tab, from its parts: its apps, its tools and its tasks
// in groups beside the one picked, its task recipes to run (one starting),
// what's said with no task run lately, and with nothing that runs here at
// all. The rows are the Runs page's own (runs/RunItem.tsx), stood in for
// here by plain ones.

const meta: Meta = { title: "Files/Runs" };
export default meta;

const none = () => {};

const row = (name: string, icon: "play" | "check" | "box" = "play") => (
  <Row key={name}>
    <Link to="/" className={rowLinkClass()}>
      <Icon name={icon} size={14} />
      <RowMain>
        <RowTitle>{name}</RowTitle>
      </RowMain>
    </Link>
  </Row>
);

const list = (
  <>
    <FilesRunsGroup title="Apps" count={2}>
      {row("web")}
      {row("storybook", "box")}
    </FilesRunsGroup>
    <FilesRunsGroup title="Tools" count={1}>
      {row("db")}
    </FilesRunsGroup>
    <FilesRunsGroup
      title="Tasks"
      count={2}
      start={
        <>
          <Button size="small" onClick={none}>
            <Icon name="play" size={11} /> Test
          </Button>
          <Button size="small" disabled>
            <Spinner size={11} /> Lint
          </Button>
        </>
      }
    >
      {row("test #3", "check")}
      {row("lint #1", "check")}
    </FilesRunsGroup>
  </>
);

export const WithOnePicked: StoryObj = {
  render: () => (
    <div style={{ height: 480, display: "flex", flexDirection: "column" }}>
      <FilesRuns
        list={list}
        panel={
          <div style={{ padding: 24 }}>
            <FilesRunsBack to="/" /> The run picked is drawn here, by the Runs page's RunPanel.
          </div>
        }
      />
    </div>
  ),
};

export const NoTaskRunLately: StoryObj = {
  render: () => (
    <div style={{ height: 360, display: "flex", flexDirection: "column" }}>
      <FilesRuns
        list={
          <FilesRunsGroup
            title="Tasks"
            start={
              <Button size="small" onClick={none}>
                <Icon name="play" size={11} /> Test
              </Button>
            }
            quiet="None of its tasks has run here lately. What a task comes to is kept a month, with the ten runs of it before."
          />
        }
        panel={
          <FilesRunsNone>
            <Empty icon="play" title="Nothing's run here lately">
              Run one of its tasks and it's shown here, with all it printed.
            </Empty>
          </FilesRunsNone>
        }
      />
    </div>
  ),
};

export const NothingRunsHere: StoryObj = {
  render: () => (
    <div style={{ height: 360, display: "flex", flexDirection: "column" }}>
      <FilesRunsNone>
        <Empty icon="play" title="Nothing runs here">
          Nothing's run in it lately, and its ws.repo.toml has no recipes.
        </Empty>
      </FilesRunsNone>
    </div>
  ),
};
