import type { Meta, StoryObj } from "@storybook/react-vite";
import { RunCame } from "./RunCame";
import { RunEnds } from "./RunEnds";
import { RunLink } from "./RunLink";
import { RunMark } from "./RunMark";
import { RunName } from "./RunName";

// What a run's line is made of, on the Runs page and in a mission's Work
// tab: its mark in each state, its name, where it answers, what a task
// came to, and the quiet line under.

const meta: Meta = { title: "Runs/RunMark" };
export default meta;

const line: React.CSSProperties = { display: "grid", gridTemplateColumns: "12px minmax(0, 1fr)", gap: 8, alignItems: "start" };
const top: React.CSSProperties = { display: "flex", flexWrap: "wrap", alignItems: "baseline", gap: "4px 10px" };

export const Lines: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 10, maxWidth: 520 }}>
      <div style={line}>
        <RunMark live up title="Up" />
        <span style={{ display: "grid", gap: 1 }}>
          <span style={top}>
            <RunName>web</RunName>
            <RunLink href="http://web.demo.localhost" title="Open it">
              web.demo.localhost
            </RunLink>
          </span>
          <RunEnds>Ends 15 minutes after you close the mission</RunEnds>
        </span>
      </div>
      <div style={line}>
        <RunMark live title="Not up yet" />
        <span style={top}>
          <RunName>api</RunName>
          <RunLink href="http://127.0.0.1:4100" title="Nothing's listening on its port yet">
            127.0.0.1:4100
          </RunLink>
        </span>
      </div>
      <div style={line}>
        <RunMark ended />
        <span style={top}>
          <RunName>worker</RunName>
          <span className="dim">exited 0, 4 minutes ago</span>
        </span>
      </div>
      {["passed", "failed", "stopped"].map((result) => (
        <div style={line} key={result}>
          <RunMark task={result} title={result} />
          <span style={{ display: "grid", gap: 1 }}>
            <span style={top}>
              <RunName to="/runs">test</RunName>
              <RunCame result={result}>{result === "passed" ? "Passed in 12s" : result === "failed" ? "Failed (exit 1) in 9s" : "Stopped after 3s"}</RunCame>
            </span>
            <RunEnds>At 3f9c2ab · check 1 of step 2</RunEnds>
          </span>
        </div>
      ))}
      <div style={line}>
        <RunMark live up title="Running" />
        <span style={top}>
          <RunName to="/runs">lint</RunName>
          <RunCame result="running">Running</RunCame>
        </span>
      </div>
    </div>
  ),
};
