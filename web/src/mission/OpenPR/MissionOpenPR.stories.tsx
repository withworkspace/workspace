import type { Meta, StoryObj } from "@storybook/react-vite";
import { takenMissions } from "../../stories/missions";
import { Button } from "../../ui/Button";
import { Icon } from "../../ui/Icon";
import { MissionOpenPR } from "./MissionOpenPR";
import { OpenPR } from "./OpenPR";

// Opening a pull request from a capsule ws gave (work-across-repos's,
// whose plan has a step that opens one): the dialog empty, written, while
// the PR writer writes it, refused; and the button alone.

const meta: Meta = { title: "Mission/MissionOpenPR" };
export default meta;

const none = () => {};
const d = () => takenMissions.find((m) => m.name === "work-across-repos")!.d;
const draw = (o: { title?: string; body?: string; draft?: boolean; writing?: boolean; error?: string }) => {
  const c = d().capsules[0];
  const step = d().stages?.find((s) => s.opensPR && s.state !== "accepted");
  return (
    <div style={{ padding: 16, background: "var(--bg)" }}>
      <Button variant="ghost" size="small" onClick={none} title="Push its branch and open a pull request, through ws">
        <Icon name="pr" size={12} /> Open pull request
      </Button>
      <MissionOpenPR
        name={`${c.repo}/${c.name}`}
        step={step?.n}
        title={o.title ?? ""}
        onTitle={none}
        body={o.body ?? ""}
        onBody={none}
        draft={o.draft ?? false}
        onDraft={none}
        writing={o.writing}
        onWrite={none}
        error={o.error}
        onClose={none}
        onOpen={none}
      />
    </div>
  );
};

export const Empty: StoryObj = { render: () => draw({}) };
export const Written: StoryObj = {
  render: () => draw({ title: "Greet by name, in the README too", body: "greet.To greets by name and falls back to world.\n\nThe README says how.", draft: true }),
};
export const Writing: StoryObj = { render: () => draw({ writing: true }) };
export const Failed: StoryObj = { render: () => draw({ title: "Greet by name", error: "refused: missions.review_before_pr: review it first" }) };
export const Closed: StoryObj = {
  render: () => (
    <div style={{ padding: 16, background: "var(--bg)" }}>
      <OpenPR d={d()} c={d().capsules[0]} reload={async () => {}} />
    </div>
  ),
};
