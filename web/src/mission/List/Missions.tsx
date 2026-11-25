import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router";
import type { Mission } from "../../api";
import { useKeys } from "../../keys";
import { useThingMenu } from "../../shell/ThingMenu";
import { Button, buttonClass } from "../../ui/Button";
import { Empty } from "../../ui/Empty";
import { GroupHead } from "../../ui/GroupHead";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { PageHead } from "../../ui/PageHead";
import { Rows } from "../../ui/Rows";
import { useTitle } from "../../ui/useTitle";
import { MissionRow } from "./MissionRow";
import { useFlash } from "./useFlash";
import { isPast, sectionsOf } from "./words";

// Missions is the missions list (/): the missions, grouped by whose court
// they're in (their headline's group), what needs you first, each saying
// where it stands in a sentence under its title. It knows which is
// selected (j and k move it), whether the past is shown, and which have
// just changed (they flash).

let lastSelected = ""; // kept across visits to a mission and back

export function Missions({ missions }: { missions: Mission[] }) {
  const menu = useThingMenu();
  useTitle("Missions");
  const navigate = useNavigate();
  const [showPast, setShowPast] = useState(false);
  const sections = useMemo(() => sectionsOf(missions, showPast), [missions, showPast]);
  const pastCount = missions.filter(isPast).length;
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
            <Button variant="ghost" size="small" onClick={() => setShowPast((p) => !p)}>
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
          <Rows role="listbox" aria-label={s.title}>
            {s.missions.map((m) => {
              const on = order[index]?.slug === m.slug;
              return (
                <MissionRow
                  key={m.slug}
                  m={m}
                  past={s.past}
                  selected={on}
                  flash={flashing.has(m.slug)}
                  onPoint={() => !on && setSelected(m.slug)}
                  onOpen={() => (lastSelected = m.slug)}
                  onContextMenu={menu(`/missions/${m.slug}`, m.title)}
                />
              );
            })}
          </Rows>
        </section>
      ))}
    </div>
  );
}
