import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import type { Anchor } from "../../ui/FloatingCard";
import { OverFrame } from "./OverFrame";
import { ThingPromote, type ThingPromotePlace } from "./ThingPromote";
import { ThingSave } from "./ThingSave";

// What ws draws over a plugin's page when it asks, from its parts: putting
// one of the plugin's things in a repo (ThingPromote) and saving it there
// (ThingSave), each in the panel by the button that asked (OverFrame), in
// every state they come to. A panel's placed by its button: here, four
// buttons in a row along the top.

const meta: Meta = { title: "Plugins/Over the frame" };
export default meta;

const none = () => {};

// At is a button's place, in the window: the panel goes under it.
const at = (column: number): Anchor => ({ isConnected: true, getBoundingClientRect: () => ({ top: 8, bottom: 10, left: 16 + column * 344, right: 16 + column * 344 }) });

function Panel({ column, label, children }: { column: number; label: string; children: ReactNode }) {
  return (
    <OverFrame anchor={at(column)} label={label} busy={false} dismiss={none}>
      {children}
    </OverFrame>
  );
}

const records: ThingPromotePlace = { key: "records:sketches/release-flow/", label: "records", where: "records:sketches/release-flow/", direct: true };
const api: ThingPromotePlace = { key: "api:docs/sketches/release-flow/", label: "api", where: "api:docs/sketches/release-flow/", direct: false };

// Promote is ThingPromote with what's typed and picked its own.
function Promote(props: { places?: ThingPromotePlace[]; pick?: number | "else"; under?: string; busy?: boolean; failed?: string; elsewhere?: string }) {
  const [pick, setPick] = useState<number | "else">(props.pick ?? 0);
  const [under, setUnder] = useState(props.under ?? "");
  const [elsewhere, setElsewhere] = useState(props.elsewhere ?? "");
  const group = under.trim().replace(/^\/+|\/+$/g, "");
  return (
    <ThingPromote
      what="Release flow"
      places={props.places}
      pick={pick}
      onPick={setPick}
      under={under}
      group={group}
      onUnder={setUnder}
      elsewhere={elsewhere}
      onElsewhere={setElsewhere}
      busy={!!props.busy}
      failed={props.failed}
      ready={pick === "else" ? !!elsewhere.trim() : !!props.places?.length}
      onPromote={none}
      onCancel={none}
    />
  );
}

// Working out where it would live; one place; one that groups its things in folders; and several to pick from.
export const Promoting: StoryObj = {
  render: () => (
    <>
      <Panel column={0} label="Put Release flow in a repo">
        <Promote />
      </Panel>
      <Panel column={1} label="Put Release flow in a repo">
        <Promote places={[records]} />
      </Panel>
      <Panel column={2} label="Put Release flow in a repo">
        <Promote places={[{ ...records, where: "records:sketches/cross-stack/release-flow/", folders: ["cross-stack", "plans", "plans/q4"] }]} under="cross-stack" />
      </Panel>
      <Panel column={3} label="Put Release flow in a repo">
        <Promote places={[records, api]} pick={1} />
      </Panel>
    </>
  ),
};

// Somewhere else, said in words: from one place, and from several; one that takes pull requests, promoting; and one that failed.
export const PromotingMore: StoryObj = {
  render: () => (
    <>
      <Panel column={0} label="Put Release flow in a repo">
        <Promote places={[records]} pick="else" elsewhere="records:drawings/" />
      </Panel>
      <Panel column={1} label="Put Release flow in a repo">
        <Promote places={[records, api]} pick="else" />
      </Panel>
      <Panel column={2} label="Put Release flow in a repo">
        <Promote places={[api]} busy />
      </Panel>
      <Panel column={3} label="Put Release flow in a repo">
        <Promote places={[]} pick="else" elsewhere="nowhere:" failed="There's no repo called nowhere in this workspace." />
      </Panel>
    </>
  ),
};

// Saving; saved; saved in a capsule, its repo taking pull requests; and merged with what had changed there.
export const Saving: StoryObj = {
  render: () => (
    <>
      <Panel column={0} label="Saving Release flow">
        <ThingSave what="Release flow" onAgain={none} onClose={none} />
      </Panel>
      <Panel column={1} label="Saving Release flow">
        <ThingSave what="Release flow" saved={{ repo: "records", path: "sketches/release-flow/sketch.excalidraw", changed: true }} onAgain={none} onClose={none} />
      </Panel>
      <Panel column={2} label="Saving Release flow">
        <ThingSave what="Release flow" saved={{ repo: "api", path: "docs/sketches/release-flow/sketch.excalidraw", changed: true, capsule: "save-release-flow" }} onAgain={none} onClose={none} />
      </Panel>
      <Panel column={3} label="Saving Release flow">
        <ThingSave what="Release flow" saved={{ repo: "records", path: "sketches/release-flow/sketch.excalidraw", changed: true, merged: true }} onAgain={none} onClose={none} />
      </Panel>
    </>
  ),
};

// Nothing to save; and one that wasn't saved.
export const SavingMore: StoryObj = {
  render: () => (
    <>
      <Panel column={0} label="Saving Release flow">
        <ThingSave what="Release flow" saved={{ repo: "records", path: "sketches/release-flow/sketch.excalidraw", changed: false }} onAgain={none} onClose={none} />
      </Panel>
      <Panel column={1} label="Saving Release flow">
        <ThingSave what="Release flow" why="records has changes of its own in sketches/release-flow/ that aren't pushed." onAgain={none} onClose={none} />
      </Panel>
    </>
  ),
};
