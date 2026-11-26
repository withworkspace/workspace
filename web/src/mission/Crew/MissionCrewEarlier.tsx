import css from "./MissionCrewEarlier.module.css";

// MissionCrewEarlier says how many of crew's entries, before those shown,
// aren't.
export function MissionCrewEarlier({ count }: { count: number }) {
  return <div className={css.earlier}>{count} earlier entries hidden.</div>;
}
