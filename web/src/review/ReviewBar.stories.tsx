import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { MenuItem, MenuText } from "../ui/Menu";
import { RepoTag } from "../ui/RepoTag";
import type { StatusInfo } from "../ui/Status";
import { Text } from "../ui/Text";
import { ReviewBar } from "./ReviewBar";
import { ReviewBase } from "./ReviewBase";
import { ReviewBy } from "./ReviewBy";
import { ReviewChangesBar } from "./ReviewChangesBar";
import { ReviewCommitCount } from "./ReviewCommitCount";
import { ReviewFact } from "./ReviewFact";
import { ReviewFacts } from "./ReviewFacts";
import { ReviewHealth } from "./ReviewHealth";
import { ReviewOpenMenu } from "./ReviewOpenMenu";
import { ReviewSideBack } from "./ReviewSideBack";
import { ReviewStand } from "./ReviewStand";

// A review's bar, in each way it's met: a pull request looked at again,
// one looked at first whose checks fail and that conflicts, one going
// into a branch that isn't the default, and a mission's capsule. Then the
// changes' bar under it, each way, and the side's way back. Point at the
// status, the commits and the pill for their cards.

const meta: Meta = { title: "Review/Bar" };
export default meta;

const none = () => {};
const changes: StatusInfo = { icon: "reply", tone: "attention", words: "You asked for changes, and they've pushed since" };
const needs: StatusInfo = { icon: "circle-dot", tone: "attention", words: "Needs a review from your team" };
const step: StatusInfo = { icon: "circle-dot", tone: "attention", words: "Step 4 is yours: read the fix" };
const commits = [
  { sha: "08b239f0abaa", subject: "Say what goes here, and offer the first invoice", author: "jun" },
  { sha: "c74af8e00000", subject: "test(invoice): an invoice whose total is a penny over", author: "jun" },
];

function Opens({ browse }: { browse?: boolean }) {
  return (
    <ReviewOpenMenu>
      {browse ? (
        <MenuItem id="browse" textValue="Browse files">
          <Icon name="file" size={13} />
          <MenuText>Browse files</MenuText>
        </MenuItem>
      ) : (
        <>
          <MenuItem id="github" textValue="Open on GitHub" keys="o">
            <Icon name="external" size={13} />
            <MenuText>Open on GitHub</MenuText>
          </MenuItem>
          <MenuItem id="dock" textValue="Dock in workspace" keys="O">
            <Icon name="box" size={13} />
            <MenuText>Dock in workspace</MenuText>
          </MenuItem>
        </>
      )}
    </ReviewOpenMenu>
  );
}

const submit = (n?: number) => (
  <Button variant="primary" size="small">
    {n ? `Submit review · ${n}` : "Submit review"} <Keys k="s" />
  </Button>
);

function Stand({ s, branch, base }: { s: StatusInfo; branch: string; base: string }) {
  return (
    <ReviewStand s={s}>
      <ReviewFacts>
        <ReviewFact term="Branch">
          <Text mono>{branch}</Text> <Icon name="arrow-right" size={11} /> <Text mono>{base}</Text>
        </ReviewFact>
        <ReviewFact term="Size">1 file · +4 −3</ReviewFact>
      </ReviewFacts>
    </ReviewStand>
  );
}

function Bars() {
  return (
    <div style={{ display: "grid", gap: 32, padding: "0 24px" }}>
      <div>
        <ReviewBar
          back={{ to: "/reviews", label: "Reviews" }}
          stand={<Stand s={changes} branch="jun/empty-state" base="main" />}
          name={<span>An empty state for a new account</span>}
          about={
            <>
              <RepoTag label="web" color="#7c6cf0" />
              <ReviewBy name="jun" />
              <ReviewCommitCount since commits={commits.slice(0, 1)} />
              <ReviewHealth checks="success" merges="clean" base="main" author="jun" />
            </>
          }
          actions={
            <>
              <Opens />
              {submit()}
            </>
          }
        />
        <ReviewChangesBar scope={{ since: true, sinceName: "your review on GitHub", onChange: none }} additions={4} deletions={3} split={{ on: false, onChange: none }} />
      </div>
      <div>
        <ReviewBar
          back={{ to: "/reviews", label: "Reviews" }}
          stand={<Stand s={needs} branch="ines/keyboard-shortcuts" base="main" />}
          name={<span>Keyboard shortcuts for the invoice list</span>}
          about={
            <>
              <RepoTag label="web" color="#7c6cf0" />
              <ReviewBy name="ines" />
              <ReviewCommitCount commits={commits.slice(0, 1)} />
              <ReviewHealth checks="failure" merges="conflicts" base="main" author="ines" />
            </>
          }
          actions={
            <>
              <Button variant="ghost" size="small">
                Not mine
              </Button>
              <Opens />
              {submit()}
            </>
          }
        />
        <ReviewChangesBar additions={10} deletions={0} split={{ on: true, onChange: none }} />
      </div>
      <ReviewBar
        back={{ to: "/reviews", label: "Reviews" }}
        stand={<Stand s={needs} branch="tomas/rate-refresh" base="tomas/rate-cache" />}
        name={<span>Refresh exchange rates in the background</span>}
        about={
          <>
            <RepoTag label="api" color="#e0a030" />
            <ReviewBy name="tomas" />
            <ReviewBase base="tomas/rate-cache" />
            <ReviewCommitCount commits={commits} />
            <ReviewHealth checks="pending" base="tomas/rate-cache" author="tomas" />
          </>
        }
        actions={
          <>
            <Opens />
            {submit()}
          </>
        }
      />
      <ReviewBar
        back={{ to: "/missions/invoice-totals", label: "Invoice totals off by a penny" }}
        stand={<Stand s={step} branch="tax-rounding" base="main" />}
        name={<Text mono>tax-rounding</Text>}
        about={
          <>
            <RepoTag label="api" color="#e0a030" />
            <ReviewCommitCount commits={commits} />
          </>
        }
        actions={
          <>
            <Opens browse />
            {submit(1)}
          </>
        }
      />
    </div>
  );
}

export const Bar: StoryObj = { render: () => <Bars /> };

export const SideBack: StoryObj = {
  render: () => (
    <div style={{ width: 280, display: "grid", gap: 16 }}>
      <ReviewSideBack to="/reviews" label="Reviews" />
      <ReviewSideBack to="/missions/invoice-totals" label="Invoice totals off by a penny, to the very last penny" />
    </div>
  ),
};
