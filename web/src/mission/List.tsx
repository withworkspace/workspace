import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { TLink } from "../ui/TLink";
import type { Crew, HeadlineGroup, Mission } from "../api";
import { useKeys } from "../keys";
import { Inline, plain } from "../ui/inline";
import { Icon } from "../ui/Icon";
import { CheckingLine } from "./checking";
import { Empty, Keys, useTitle } from "../ui/kit";
import { crewStatus, headlineStatus, Status } from "../ui/status";
import { Time } from "../ui/time";
import { Chip } from "../ui/Chip";
import { Button, buttonClass } from "../ui/Button";
import { PageHead } from "../ui/PageHead";
import { GroupHead } from "../ui/GroupHead";
import { Row, RowLine, RowMain, RowMeta, RowSub, RowTitle, Rows, rowLinkClass, rowStatusClass } from "../ui/Rows";

// The missions, grouped by whose court they're
// in (their headline's group): what needs you first. Each says where it
// stands in a sentence, under its title.

const groups: { group: HeadlineGroup; title: string }[] = [
  { group: "needs-you", title: "Needs you" },
  { group: "in-progress", title: "In progress" },
  { group: "waiting", title: "Waiting" },
  { group: "up-next", title: "Up next" },
];
const past = (m: Mission) => m.headline.group === "past";

// onRoute is where a mission is on its route, in a few words: "Step 2 of
// 7", or "At Z" once every step's done; its whole sentence is the title.
const onRoute = (at: string) => /^(?:Step|Stage) \d+ of \d+/.exec(at)?.[0] ?? (/^Every step done/.test(at) ? "At Z" : "");

let lastSelected = ""; // kept across visits to a mission and back

export const listKeys: [string, string][] = [
  ["j / k", "Next / previous mission"],
  ["g / G", "First / last"],
  ["⏎ / o", "Open it"],
  ["⇧A", "Show or hide past missions"],
];

export function MissionList({ missions }: { missions: Mission[] }) {
  useTitle("Missions");
  const navigate = useNavigate();
  const [showPast, setShowPast] = useState(false);
  const sections = useMemo(() => {
    const placed = new Set<string>();
    const out: { title: string; missions: Mission[]; past?: boolean }[] = [];
    for (const g of groups) {
      const ms = missions.filter((m) => !placed.has(m.slug) && m.headline.group === g.group);
      ms.forEach((m) => placed.add(m.slug));
      if (ms.length) out.push({ title: g.title, missions: ms });
    }
    if (showPast) {
      const ms = missions.filter((m) => !placed.has(m.slug) && past(m));
      if (ms.length) out.push({ title: "Past", missions: ms, past: true });
    }
    return out;
  }, [missions, showPast]);
  const pastCount = missions.filter(past).length;
  const order = sections.flatMap((s) => s.missions);
  const [selected, setSelected] = useState(lastSelected);
  const index = Math.max(0, order.findIndex((m) => m.slug === selected));
  const select = (i: number) => {
    const m = order[Math.min(Math.max(i, 0), order.length - 1)];
    if (m) {
      setSelected(m.slug);
      lastSelected = m.slug;
      document.getElementById(`mission-${m.slug}`)?.scrollIntoView({ block: "nearest" });
    }
  };
  const open = () => order[index] && navigate(`/missions/${order[index].slug}`);
  useKeys({
    j: () => select(index + 1),
    ArrowDown: () => select(index + 1),
    k: () => select(index - 1),
    ArrowUp: () => select(index - 1),
    g: () => select(0),
    G: () => select(order.length - 1),
    Enter: open,
    o: open,
    A: () => setShowPast((p) => !p),
  });

  // A mission whose headline changes while you look flashes, so a change
  // isn't missed.
  const flashing = useFlash(missions);

  const head = (
    <PageHead
      title="Missions"
      actions={
        <>
          {pastCount > 0 && (
            <Button variant="ghost" size="small" onClick={() => setShowPast((p) => !p)} aria-pressed={showPast}>
              {showPast ? "Hide past" : `Show past (${pastCount})`} <Keys k="⇧A" />
            </Button>
          )}
          <Link to="/conversations?new=1" className={buttonClass()}>
            <Icon name="plus" size={14} /> New mission
          </Link>
        </>
      }
    />
  );

  if (!order.length) {
    return (
      <div className="page">
        {head}
        <Empty
          icon="flag"
          title={missions.length ? "Nothing on right now" : "No missions yet"}
          action={
            <Link to="/conversations?new=1" className={buttonClass({ variant: "primary" })}>
              Start with a conversation
            </Link>
          }
        >
          A mission is a piece of work crew carry out for you, stage by stage. Describe what you want in a conversation and the agent proposes one, or run{" "}
          <code>ws mission new</code>.
        </Empty>
      </div>
    );
  }
  return (
    <div className="page">
      {head}
      {sections.map((s) => (
        <section key={s.title} aria-label={s.title}>
          <GroupHead count={s.missions.length}>
            {s.title}
          </GroupHead>
          <Rows className="missions" role="listbox" aria-label={s.title}>
            {s.missions.map((m) => {
              const st = headlineStatus(m.headline);
              const live = m.crew.filter((c) => !c.ended);
              const on = order[index]?.slug === m.slug;
              return (
                <Row
                  selected={on} flash={flashing.has(m.slug)}
                  key={m.slug}
                  id={`mission-${m.slug}`}
                  role="option"
                  aria-selected={on}
                  className="mission-row"
                  onMouseMove={() => !on && setSelected(m.slug)}
                >
                  <TLink to={`/missions/${m.slug}`} className={rowLinkClass("mission-link")} onClick={() => (lastSelected = m.slug)}>
                    <Status s={st} iconOnly size={16} className={rowStatusClass()} />
                    <RowMain>
                      <RowLine>
                        <RowTitle>{m.title}</RowTitle>
                        {live.map((c) => (
                          <CrewChip key={c.id} crew={c} />
                        ))}
                      </RowLine>
                      {m.headline.text && (
                        <RowSub className="mission-sub" title={plain(m.headline.text)}>
                          {m.headline.checking ? <CheckingLine c={m.headline.checking} /> : <Inline text={m.headline.text} lines={1} />}
                        </RowSub>
                      )}
                    </RowMain>
                    <Chip tone={st.tone} className="mission-chip">{st.words}</Chip>
                    {m.at && onRoute(m.at) && (
                      <RowMeta title={m.at}>
                        {onRoute(m.at)}
                      </RowMeta>
                    )}
                    {m.capsules.length > 0 && (
                      <RowMeta title={m.capsules.map((c) => `${c.repo}/${c.name}`).join("\n")}>
                        <Icon name="box" size={13} />
                        {m.capsules.length === 1 ? `${m.capsules[0].repo}/${m.capsules[0].name}` : `${m.capsules.length} capsules`}
                      </RowMeta>
                    )}
                    <Time iso={m.updated || m.created} as={s.past ? "stamp" : "age"} className="age" />
                  </TLink>
                </Row>
              );
            })}
          </Rows>
        </section>
      ))}
    </div>
  );
}

function CrewChip({ crew }: { crew: Crew }) {
  const st = crewStatus(crew);
  return (
    <Chip tone={st.tone} className="crew-chip" title={`${crew.id}: ${st.words}`}>
      <Icon name={st.icon} size={12} className={st.live ? (st.icon === "spinner" ? "spin" : "pulse") : undefined} />
      {crew.id}
    </Chip>
  );
}

// useFlash is the missions whose headline changed since the last render,
// for a moment.
function useFlash(missions: Mission[]): Set<string> {
  const before = useRef<Map<string, string> | null>(null);
  const [flashing, setFlashing] = useState<Set<string>>(new Set());
  useEffect(() => {
    // What's running while ws checks a step changes as it prints: it
    // flashes as each check starts, not with each line.
    const now = new Map(missions.map((m) => [m.slug, `${m.headline.kind}|${m.headline.checking ? m.headline.since : m.headline.text}`]));
    if (before.current) {
      const changed = [...now].filter(([slug, h]) => before.current!.has(slug) && before.current!.get(slug) !== h).map(([slug]) => slug);
      if (changed.length) {
        setFlashing(new Set(changed));
        const t = setTimeout(() => setFlashing(new Set()), 1700);
        before.current = now;
        return () => clearTimeout(t);
      }
    }
    before.current = now;
  }, [missions]);
  return flashing;
}
