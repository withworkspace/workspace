import { useEffect, useState } from "react";
import { type Capsule, getMission, type Mission } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { OverviewMission } from "./OverviewMission";
import { Text } from "../ui/Text";
import { Prose } from "../ui/Prose";

// MissionContext is a capsule's overview: the mission it's for, where that
// stands, and its brief, which says what the capsule's work is for.
export function MissionContext({ mission, capsule }: { mission: Mission; capsule: Capsule }) {
  const [brief, setBrief] = useState<string | null>();
  useEffect(() => {
    let stopped = false;
    getMission(mission.slug).then(
      (d) => !stopped && setBrief(d.brief),
      () => !stopped && setBrief(null),
    );
    return () => {
      stopped = true;
    };
  }, [mission.slug]);
  const h = mission.headline;
  return (
    <>
      <h2>Mission</h2>
      <OverviewMission slug={mission.slug} title={mission.title} repo={capsule.repo} name={capsule.name} branch={capsule.branch} />
      {h.text && <Text as="p" dim>{h.text}</Text>}
      <h2>Brief</h2>
      {brief === undefined ? (
        <Text as="p" dim>Loading brief…</Text>
      ) : brief === null ? (
        <Text as="p" dim>Could not load brief.</Text>
      ) : brief.trim() ? (
        <Prose>
          <Markdown text={brief} />
        </Prose>
      ) : (
        <Text as="p" dim>No brief yet.</Text>
      )}
    </>
  );
}
