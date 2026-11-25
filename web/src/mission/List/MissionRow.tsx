import type { MouseEvent } from "react";
import type { Mission } from "../../api";
import { crewStatus, headlineStatus } from "../../ui/statusOf";
import { plain } from "../../ui/text";
import { CheckingLine } from "../Checking/CheckingLine";
import { MissionListCrew } from "./MissionListCrew";
import { MissionListRow } from "./MissionListRow";
import { onRoute } from "./words";

// MissionRow says how a mission is drawn in the missions list, as a
// MissionListRow: its headline's status and sentence (or what ws is
// running while it checks a step), its crew at work, where it is on its
// route and its capsules.
export function MissionRow({ m, past, selected, flash, onPoint, onOpen, onContextMenu }: { m: Mission; past?: boolean; selected: boolean; flash: boolean; onPoint: () => void; onOpen: () => void; onContextMenu?: (e: MouseEvent) => void }) {
  const route = m.at ? onRoute(m.at) : "";
  return (
    <MissionListRow
      id={`mission-${m.slug}`}
      to={`/missions/${m.slug}`}
      status={headlineStatus(m.headline)}
      title={m.title}
      crew={m.crew
        .filter((c) => !c.ended)
        .map((c) => {
          const st = crewStatus(c);
          return <MissionListCrew key={c.id} id={c.id} tone={st.tone} icon={st.icon} live={st.live} says={st.words} />;
        })}
      sub={m.headline.text ? { title: plain(m.headline.text), checking: m.headline.checking ? <CheckingLine c={m.headline.checking} /> : undefined, said: m.headline.text } : undefined}
      route={route ? { words: route, title: m.at! } : undefined}
      capsules={
        m.capsules.length > 0
          ? { words: m.capsules.length === 1 ? `${m.capsules[0].repo}/${m.capsules[0].name}` : `${m.capsules.length} capsules`, title: m.capsules.map((c) => `${c.repo}/${c.name}`).join("\n") }
          : undefined
      }
      time={{ iso: m.updated || m.created, as: past ? "stamp" : "age" }}
      selected={selected}
      flash={flash}
      onPoint={onPoint}
      onOpen={onOpen}
      onContextMenu={onContextMenu}
    />
  );
}
