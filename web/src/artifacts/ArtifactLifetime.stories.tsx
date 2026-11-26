import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Rows } from "../ui/Rows";
import { ArtifactDiscard } from "./ArtifactDiscard";
import { ArtifactDiscardedNote } from "./ArtifactDiscardedNote";
import { ArtifactExpiryNote } from "./ArtifactExpiryNote";
import { ArtifactPromote } from "./ArtifactPromote";
import { ArtifactRow } from "./ArtifactRow";
import { days, discarded, expired, live, none, Page } from "./samples";
import { type IntoKind, type Lives, promoteBlocked } from "./shown";
import { Text } from "../ui/Text";

// An artifact's lifetime, from its parts: a live one in its last week, in
// the list (when it expires, with Promote beside it) and on its page; one
// that's expired, quiet until the daemon discards it; one discarded; and
// the dialogs that promote one (the first time, where it's to live is
// asked; once it lives in a repo, it says what lands there) and discard
// one.

const meta: Meta = { title: "Artifacts/Lifetime" };
export default meta;

const version = (
  <Chip tone="quiet" title="1 version">
    v1
  </Chip>
);

const here = (id: string) => (
  <>
    <span title="On this machine only">
      <Icon name="monitor" size={11} /> This machine
    </span>
    <Text mono>{id}</Text>
  </>
);

const soonRow = (
  <ArtifactRow
    to="/artifacts/a5"
    option
    status={live}
    title="Invoice footer"
    chips={
      <>
        {version}
        <Chip tone="attention" title="No new version since 30/10/2026: promote it to keep it">
          <Icon name="clock" size={11} /> expires in 3 days
        </Chip>
      </>
    }
    note="Where the totals sit on a narrow page."
    when={{ by: "by you", at: days(26), title: "v1 by you" }}
    onPromote={none}
  >
    {here("a5")}
  </ArtifactRow>
);

const expiredRow = (
  <ArtifactRow
    to="/artifacts/a8"
    option
    dimmed
    status={expired}
    title="Sidebar widths"
    chips={
      <>
        {version}
        <Chip tone={expired.tone}>{expired.words}</Chip>
      </>
    }
    note="Three widths of the sidebar, side by side."
    when={{ by: "by you", at: days(32), title: "v1 by you" }}
    onPromote={none}
  >
    {here("a8")}
  </ArtifactRow>
);

export const InTheList: StoryObj = {
  render: () => (
    <div className="page narrow">
      <Rows role="listbox" aria-label="Artifacts">
        {soonRow}
        {expiredRow}
      </Rows>
    </div>
  ),
};

export const AboutToEnd: StoryObj = {
  render: () => <Page title="Invoice footer" versions={[{ n: 1, by: "by you", at: days(26), note: "Where the totals sit on a narrow page." }]} notes={<ArtifactExpiryNote words="expires in 3 days" expired={false} />} />,
};

export const Ended: StoryObj = {
  render: () => (
    <Page
      title="Sidebar widths"
      chip={
        <Chip tone={expired.tone}>
          <Icon name={expired.icon} size={11} />
          {expired.words}
        </Chip>
      }
      versions={[{ n: 1, by: "by you", at: days(32), note: "Three widths of the sidebar, side by side." }]}
      notes={<ArtifactExpiryNote words="expired" expired />}
    />
  ),
};

export const Discarded: StoryObj = {
  render: () => (
    <Page
      title="Old pager"
      chip={
        <Chip tone={discarded.tone}>
          <Icon name={discarded.icon} size={11} />
          {discarded.words}
        </Chip>
      }
      versions={[{ n: 1, by: "by you", at: days(9), note: "A pager with numbers. Nobody chose it." }]}
      notes={<ArtifactDiscardedNote at={days(2)} by="you" reason="Nobody chose it, and the list scrolls now" until={new Date(Date.now() + 28 * 24 * 60 * 60 * 1000).toLocaleDateString()} />}
    />
  ),
};

function Promote({ home, lives: first = "offered", kind: firstKind = "nothing", error }: { home?: { where: string; repo: string; version: number }; lives?: Lives; kind?: IntoKind; error?: string }) {
  const [n, setN] = useState(2);
  const [kind, setKind] = useState<IntoKind>(firstKind);
  const [text, setText] = useState("");
  const [lives, setLives] = useState<Lives>(home ? "home" : first);
  const [other, setOther] = useState("records:");
  const offered = home ? undefined : "records:artifacts/2026-11-25-where-export-goes/";
  const folder = lives === "offered" ? (offered ?? "") : lives === "other" ? other : "";
  return (
    <ArtifactPromote
      title="Where Export goes"
      ready
      versions={[
        { n: 2, note: "Export under the list, beside the count it exports." },
        { n: 1, note: "Export at the end of the toolbar." },
      ]}
      version={n}
      onVersion={setN}
      kind={kind}
      onKind={setKind}
      text={text}
      onText={setText}
      missions={[
        { slug: "export-invoices-as-csv", title: "Export invoices as CSV" },
        { slug: "dark-mode-for-the-dashboard", title: "Dark mode for the dashboard" },
      ]}
      home={home}
      offered={offered}
      lives={lives}
      onLives={setLives}
      other={other}
      onOther={setOther}
      folder={folder}
      branch="main"
      blocked={promoteBlocked(kind, text, lives, folder)}
      error={error}
      busy={false}
      onGo={none}
      onClose={none}
    />
  );
}

export const PromoteTheFirstTime: StoryObj = { render: () => <Promote /> };
export const PromoteToStayHere: StoryObj = { render: () => <Promote lives="here" /> };
export const PromoteIntoAnotherFolder: StoryObj = { render: () => <Promote lives="other" kind="mission" /> };
export const PromoteAgain: StoryObj = { render: () => <Promote home={{ where: "records:artifacts/2026-11-20-where-export-goes/", repo: "records", version: 1 }} error="The records repo has changes that aren't yours on main." /> };

function Discard({ home }: { home?: { repo: string; where: string } }) {
  const [why, setWhy] = useState("");
  return <ArtifactDiscard title="Where Export goes" id="a1" home={home} branch="main" why={why} onWhy={setWhy} busy={false} onGo={none} onClose={none} />;
}

export const DiscardOne: StoryObj = { render: () => <Discard /> };
export const DiscardOneInARepo: StoryObj = { render: () => <Discard home={{ repo: "records", where: "records:artifacts/2026-11-20-where-export-goes/" }} /> };
