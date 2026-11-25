import type { Meta, StoryObj } from "@storybook/react-vite";
import type { MissionCard, MissionDetail, Repo, Workspace } from "../api";
import { RefsProvider } from "../refs/known";
import { useWorkspaceStore } from "../workspace/store";
import { serve } from "../stories/api";
import courtCases from "./fixtures/court.json";
import allKinds from "./fixtures/kinds.json";
import { MissionData } from "./MissionData";
import { SummaryLead } from "./Summary/SummaryLead";
import { GroupHead } from "../ui/GroupHead";
import { Text } from "../ui/Text";

// A mission's banners: where it stands, as its Summary leads with it.
//
// Every card here is one ws made (mission.Cards), drawn by the part the
// mission's page draws it with. None is written by hand:
//
//   - missions: missions as the API gave them, from seeded tries
//     (fixtures/<try>/, which scripts/story-fixtures.mjs takes again), with
//     the step they're on and what their crew last said;
//   - the court's cases: the cards of every case in the court's table of
//     tests (fixtures/court.json, written by the same script), which has
//     each kind of card there is. They're cards alone, with no mission
//     round them, so no step line.
const missions = import.meta.glob<MissionDetail>("./fixtures/*/missions/*.json", { eager: true, import: "default" });
const workspaces = import.meta.glob<Workspace>("./fixtures/*/workspace.json", { eager: true, import: "default" });
const repos = import.meta.glob<{ repos: Repo[] }>("./fixtures/*/repos.json", { eager: true, import: "default" });

// The tries' repos together answer /api/repos: a name in a banner links to
// its repo whichever try the mission came from.
serve("/api/repos", { repos: Object.values(repos).flatMap((r) => r.repos) });

// The tries' workspaces together are the one the store holds
// (workspace/store.ts): a name in a banner is drawn as its own try says
// it stands, whichever try the mission came from. (Where it is, and what
// it's called, are the first try's.)
const tries = Object.values(workspaces);
useWorkspaceStore.setState({
  workspace: {
    ...tries[0],
    missions: tries.flatMap((w) => w.missions),
    conversations: tries.flatMap((w) => w.conversations),
    pages: tries.flatMap((w) => w.pages ?? []),
    artifacts: tries.flatMap((w) => w.artifacts ?? []),
    spaces: tries.flatMap((w) => w.spaces ?? []),
  },
});

// A mission as it was taken: which try it's from, and the name it was
// taken under (its slug, or <slug>.<as> for one taken again after
// something was done to it).
type Taken = { from: string; name: string; d: MissionDetail };

const tryOf = (path: string) => path.split("/")[2];

const taken: Taken[] = Object.entries(missions).map(([path, d]) => ({
  from: tryOf(path),
  name: path.split("/").pop()!.replace(/\.json$/, ""),
  d,
}));

// A case of the court's, as a mission with nothing but its cards.
const cased: Taken[] = (courtCases as unknown as { name: string; cards: MissionCard[] }[]).map((c) => ({
  from: "court",
  name: c.name,
  d: { slug: "s", title: c.name, cards: c.cards, headlines: [], stages: [], crew: [], tasks: [], events: [], decisions: [], capsules: [], artifacts: [] } as unknown as MissionDetail,
}));

const kindRank = (k: string) => {
  const i = allKinds.indexOf(k);
  return i < 0 ? allKinds.length : i;
};
const lead = (t: Taken): MissionCard | undefined => t.d.cards[0];
const pressing = (ts: Taken[]) => [...ts].sort((a, b) => kindRank(lead(a)?.kind ?? "") - kindRank(lead(b)?.kind ?? "") || a.name.localeCompare(b.name));
const named = (name: string) => taken.find((t) => t.name === name);

// Banner is a mission's cards as its Summary draws them, with what the
// page has around them: the mission, and the workspace's names.
function Banner({ t }: { t: Taken }) {
  return (
    <RefsProvider>
      <MissionData.Provider value={t.d}>
        <SummaryLead d={t.d} reload={async () => {}} />
      </MissionData.Provider>
    </RefsProvider>
  );
}

// What ws said of each card, over it: its kind and tone, what's folded
// into it, and what it offers, in order (* the one meant to move it, ~
// the quiet ones).
function Facts({ t }: { t: Taken }) {
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: "4px 16px", fontSize: "var(--text-sm)", margin: "0 0 8px" }}>
      <strong>{t.d.title}</strong>
      <span style={{ color: "var(--dim)" }}>
        {t.from}/{t.name}
      </span>
      {t.d.cards.map((c, i) => (
        <span key={i}>
          <span style={{ color: "var(--dim)" }}>{i === 0 ? "lead" : "notice"}</span> <code>{c.kind}</code> · <code>{c.tone}</code>
          {c.folded && (
            <>
              {" "}
              <span style={{ color: "var(--dim)" }}>with</span> <code>{c.folded}</code>
            </>
          )}{" "}
          <code>{[c.primary && `*${c.primary.do}`, ...(c.more ?? []).map((a) => `${a.quiet ? "~" : ""}${a.do}`)].filter(Boolean).join(" ") || "nothing to press"}</code>
        </span>
      ))}
    </div>
  );
}

function Shown({ t }: { t: Taken }) {
  return (
    <div style={{ margin: "0 0 28px" }} data-banner={`${t.from}/${t.name}`}>
      <Facts t={t} />
      <Banner t={t} />
    </div>
  );
}

// The page's own column: a mission's Summary is about this wide beside its
// details.
function Column({ children }: { children: React.ReactNode }) {
  return <div style={{ maxWidth: 1000, margin: "0 auto", padding: "24px 24px 80px" }}>{children}</div>;
}

// Which kinds of card are here, and which aren't.
function Coverage({ of, what }: { of: Taken[]; what: string }) {
  const have = new Set(of.flatMap((t) => t.d.cards.map((c) => c.kind as string)));
  const missing = allKinds.filter((k) => !have.has(k));
  return (
    <p style={{ margin: "0 0 28px", fontSize: "var(--text-sm)" }}>
      {of.length} {what}, with {allKinds.length - missing.length} of the {allKinds.length} kinds of card.
      {missing.length > 0 && (
        <>
          {" "}
          <Text dim>Not here:</Text>{" "}
          {missing.map((k) => (
            <code key={k} style={{ marginRight: 6 }}>
              {k}
            </code>
          ))}
        </>
      )}
    </p>
  );
}

function Groups({ of }: { of: Taken[] }) {
  const sorted = pressing(of);
  const kinds = [...new Set(sorted.map((t) => lead(t)?.kind ?? ""))];
  return (
    <>
      {kinds.map((k) => (
        <section key={k}>
          <GroupHead>{k}</GroupHead>
          {sorted
            .filter((t) => (lead(t)?.kind ?? "") === k)
            .map((t) => (
              <Shown key={`${t.from}/${t.name}`} t={t} />
            ))}
        </section>
      ))}
    </>
  );
}

const meta: Meta = { title: "Mission/Banners" };
export default meta;

// Every kind of card, from the court's cases, most pressing first.
export const EveryKind: StoryObj = {
  name: "Every kind",
  render: () => (
    <Column>
      <Coverage of={cased} what="cases of the court's" />
      <Groups of={cased} />
    </Column>
  ),
};

// Every mission taken from a seeded try, whole: with its step and its crew.
export const Missions: StoryObj = {
  render: () => (
    <Column>
      <Coverage of={taken} what="missions, as ws gave them" />
      <Groups of={taken} />
    </Column>
  ),
};

// A Wait step, each way one's passed: on you or on someone else, watched
// by a check or not, and with the orchestrator asking at it.
export const Waits: StoryObj = {
  render: () => (
    <Column>
      {[
        ["On you, nobody watching", "cut-the-tag"],
        ["On you, ws watching", "tag-and-publish"],
        ["On someone else, nobody watching", "platform-deploy"],
        ["On someone else, ws watching", "on-the-registry"],
        ["On someone else, nobody watching, and the orchestrator asks", "platform-deploy.asked"],
        ["On someone else, ws watching, and the orchestrator asks", "on-the-registry.asked"],
      ].map(([what, name]) => {
        const t = named(name);
        return (
          <section key={name}>
            <GroupHead>{what}</GroupHead>
            {t ? <Shown t={t} /> : <Text as="p" dim>No mission {name} among the fixtures.</Text>}
          </section>
        );
      })}
    </Column>
  ),
};

// One mission's banner, alone: pick which.
const all = [...pressing(taken), ...pressing(cased)];
export const One: StoryObj<{ mission: string }> = {
  args: { mission: all[0] ? `${all[0].from}/${all[0].name}` : "" },
  argTypes: { mission: { control: "select", options: all.map((t) => `${t.from}/${t.name}`) } },
  render: ({ mission }) => {
    const t = all.find((x) => `${x.from}/${x.name}` === mission);
    return <Column>{t ? <Shown t={t} /> : <Text as="p" dim>Pick a mission.</Text>}</Column>;
  },
};
