import type { Meta, StoryObj } from "@storybook/react-vite";
import { type ReactNode, useState } from "react";
import { FindPane } from "../shell/Palette/FindPane";
import { finderBox } from "../ui/Finder";
import { HoverCardLine } from "../ui/HoverCard";
import type { Named } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefChip } from "./RefChip";
import { RefExcerpt, RefExcerptLine } from "./RefExcerpt";
import { RefFloat } from "./RefFloat";
import { RefCommitCard } from "./RefCommitCard";
import { RefGo } from "./RefGo";
import { RefIssueCard } from "./RefIssueCard";
import { RefLabel } from "./RefLabel";
import { RefName } from "./RefName";
import type { ThingRecord } from "./thing";
import { ThingView } from "./ThingView";
import { markdownClass } from "../markdown/LazyMarkdown";
import { cx } from "../ui/cx";
import { Text } from "../ui/Text";

// Refs, from their parts: names in what's read, of each kind and shape,
// with what's said after them, in markdown and in words that say
// data-refs (and nowhere, where they're plain links); a ref's card over
// the page, for a file (lines named, more after them), a repo, a pull
// request, an issue (as GitHub says it is) and a commit; a file's lines in
// Find's pane; and a plugin's thing, as its
// plugin has it drawn on a card and in Find's pane.

const meta: Meta = { title: "Refs/Refs" };
export default meta;

const none = () => {};

const names = (
  <>
    The rate limit is <RefName kind="mission" shape="words" to="/missions/rate-limit" dataRef="rate-limit">rate-limit</RefName>
    <RefLabel tone="progress">In progress</RefLabel>, its design in <RefName kind="file" shape="words" to="/files/api" dataRef="api:limits.go">api:limits.go</RefName>, asked
    for in <RefName kind="talk" shape="words" to="/conversations/w1" dataRef="w1">w1</RefName>
    <RefLabel tone="quiet">Limits per key</RefLabel> and reviewed in <RefName kind="pr" shape="words" to="/reviews/api/41" dataRef="api#41">api#41</RefName>, for the issue{" "}
    <RefName kind="issue" shape="words" href="https://github.com/sorrel/api/issues/38" dataRef="api#38">api#38</RefName>, since{" "}
    <RefName kind="commit" shape="words" href="https://github.com/sorrel/api/commit/4f2a9c1e8b7d6c5a4f3e2d1c0b9a8f7e6d5c4b3a" title="api@4f2a9c1e8b7d6c5a4f3e2d1c0b9a8f7e6d5c4b3a" dataRef="api@4f2a9c1e8b7d6c5a4f3e2d1c0b9a8f7e6d5c4b3a">
      api@4f2a9c1
    </RefName>
    . It lives in{" "}
    <RefName kind="repo" shape="pill" icon="box" accent="#6a8fd8" to="/files/api" title="api" dataRef="api">
      api
    </RefName>
    , and follows{" "}
    <RefName kind="file" shape="words" words icon="file" to="/files/records" dataRef="records:adr/0001.md">
      the money ADR
    </RefName>
    , and <RefName kind="thing" shape="words" onClick={none} dataRef="RAM-12">RAM-12</RefName>, inline.
  </>
);

export const Names: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gap: 18, maxWidth: 720 }}>
      <div>
        <Text as="p" dim>In markdown</Text>
        <div className={cx("markdown", markdownClass)}>
          <p>{names}</p>
        </div>
      </div>
      <div>
        <Text as="p" dim>In words that say data-refs (Inline, a list of refs)</Text>
        <p data-refs="">{names}</p>
      </div>
      <div>
        <Text as="p" dim>Anywhere else</Text>
        <p>{names}</p>
      </div>
    </div>
  ),
};

const code = ["func limit(key string) bool {", "\tw := windows.of(key)", "\tif w.count() >= perMinute {", "\t\treturn false", "\t}", "\tw.add(now())", "\treturn true", "}"];

// A card held open over the words it's about.
function Open({ words, hint, children }: { words: string; hint?: boolean; children: ReactNode }) {
  const [el, setEl] = useState<HTMLElement | null>(null);
  return (
    <div style={{ height: 360 }}>
      <Text dim ref={setEl}>
        {words}
      </Text>
      {el && (
        <RefFloat anchor={el} onEnter={none} onLeave={none} hint={hint}>
          {children}
        </RefFloat>
      )}
    </div>
  );
}

const fileCard = (raised?: boolean) => (
  <>
    <RefCardHead icon="file" title="api/limits.go" mono />
    <HoverCardLine dim>api, default branch, lines 3 to 4</HoverCardLine>
    <RefExcerpt more raised={raised}>
      {code.map((l, i) => (
        <RefExcerptLine key={i} n={i + 1} named={i === 2 || i === 3}>
          {l}
        </RefExcerptLine>
      ))}
    </RefExcerpt>
    <RefGo to="/files/api">Open it in Files</RefGo>
  </>
);

export const Card: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 24 }}>
      <Open words="A file, with lines named" hint>
        {fileCard()}
      </Open>
      <Open words="A repo">
        <RefCardHead icon="box" title="Sorrel API" aside="api" />
        <HoverCardLine dim>Default branch main, 2 behind</HoverCardLine>
        <HoverCardLine dim>2 capsules: rate-limit, csv-export</HoverCardLine>
        <RefGo to="/files/api">Open it in Files</RefGo>
      </Open>
      <Open words="A pull request">
        <RefCardHead icon="pr" title="Limit each key to 100 requests a minute" />
        <HoverCardLine dim>api#41, by ana</HoverCardLine>
        <HoverCardLine>
          Changes asked for
          <RefChip tone="danger">checks fail</RefChip>
        </HoverCardLine>
        <HoverCardLine dim>+212 −18, 6 files</HoverCardLine>
        <RefGo to="/reviews/api/41">Review it</RefGo>
      </Open>
      <Open words="A folder">
        <RefCardHead icon="layers" title="api/limits/" mono />
        <HoverCardLine dim>api, rate-limit, 3 things in it</HoverCardLine>
        <RefExcerpt>
          {["window/", "limits.go", "limits_test.go"].map((n) => (
            <RefExcerptLine key={n}>{n}</RefExcerptLine>
          ))}
        </RefExcerpt>
        <RefGo href="https://example.com">Open it outside</RefGo>
      </Open>
    </div>
  ),
};

// An issue's card, as GitHub says the number is: an issue (or a pull
// request that isn't open), where GitHub couldn't be asked, and still
// asking; one that's an
// open pull request is drawn as a pull request's card (Card). A commit's
// card, by a short sha and a full one; and both where the repo isn't on
// GitHub, whose link goes to its files.
export const IssueAndCommit: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 24 }}>
      <Open words="An issue" hint>
        <RefIssueCard r={{ kind: "issue", repo: "api", number: 38 }} href="https://github.com/sorrel/api/issues/38" is={{ is: "issue" }} />
      </Open>
      <Open words="An issue, GitHub couldn't be asked">
        <RefIssueCard r={{ kind: "issue", repo: "api", number: 38 }} href="https://github.com/sorrel/api/issues/38" is={{ is: "unknown" }} />
      </Open>
      <Open words="An issue, asking GitHub">
        <RefIssueCard r={{ kind: "issue", repo: "api", number: 38 }} href="https://github.com/sorrel/api/issues/38" is={{ is: "asking" }} />
      </Open>
      <Open words="A commit, by a short sha">
        <RefCommitCard r={{ kind: "commit", repo: "api", commit: "4f2a9c1" }} href="https://github.com/sorrel/api/commit/4f2a9c1" />
      </Open>
      <Open words="A commit, by its full sha">
        <RefCommitCard r={{ kind: "commit", repo: "api", commit: "4f2a9c1e8b7d6c5a4f3e2d1c0b9a8f7e6d5c4b3a" }} href="https://github.com/sorrel/api/commit/4f2a9c1e8b7d6c5a4f3e2d1c0b9a8f7e6d5c4b3a" />
      </Open>
      <Open words="A commit, of a repo not on GitHub">
        <RefCommitCard r={{ kind: "commit", repo: "records", commit: "4f2a9c1" }} href="/files/records" />
      </Open>
    </div>
  ),
};

export const InFind: StoryObj = {
  render: () => (
    <div className="page">
      <div className={finderBox} style={{ display: "flex", width: "auto", height: 320 }}>
        <FindPane>{fileCard(true)}</FindPane>
      </div>
    </div>
  ),
};

const issue: Named = {
  plugin: "jira",
  what: "issue",
  prefixes: ["RAM-"],
  title: "Jira issue",
  fields: [
    { name: "title", type: "title" },
    { name: "status", type: "state", label: "Status" },
    { name: "assignee", type: "text", label: "Assignee" },
    { name: "updated", type: "time", label: "Updated" },
    { name: "body", type: "markdown" },
  ],
  views: {
    card: [
      { block: "title", field: "title" },
      { block: "subtitle", field: "key" },
      { block: "badge", field: "status" },
      { block: "field", field: "assignee" },
      { block: "markdown", field: "body", lines: 4 },
    ],
    pane: [
      { block: "badge", field: "status" },
      { block: "badge", field: "assignee" },
      { block: "title", field: "title" },
      { block: "field", field: "updated" },
      { block: "markdown", field: "body" },
    ],
  },
};
const record: ThingRecord = {
  key: "RAM-12",
  fields: {
    title: "Token refresh is flaky on a slow clock, and the retry doubles the wait each time",
    status: "In Progress",
    assignee: "Dana",
    updated: "2026-11-18T10:42:00Z",
    body: "## What happens\n\nThe **clock** should be injected. Today it's read from the machine, so a test that waits a minute waits a minute.\n\n- Inject it\n- Wait on it in tests\n- Say so in the guide\n\nThen the flake goes.",
  },
};

export const Thing: StoryObj = {
  render: () => (
    <div className="page" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24 }}>
      <Open words="On a card, kept to four lines">
        <ThingView named={issue} record={record} surface="card" />
      </Open>
      <div className={finderBox} style={{ display: "flex", width: "auto", height: 360 }}>
        <FindPane scrolls>
          <ThingView named={issue} record={record} surface="pane" />
        </FindPane>
      </div>
    </div>
  ),
};
