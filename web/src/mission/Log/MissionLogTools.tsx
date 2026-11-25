import { Input } from "../../ui/Input";
import { Segment, Segmented } from "../../ui/Segmented";
import css from "./MissionLogTools.module.css";

// MissionLogTools is over a mission's log: whose events it shows, and what
// it's searched for.
export function MissionLogTools<F extends string>({
  filters,
  filter,
  onFilter,
  q,
  onQ,
}: {
  filters: readonly { id: F; label: string }[];
  filter: F;
  onFilter: (f: F) => void;
  q: string;
  onQ: (q: string) => void;
}) {
  return (
    <div className={css.tools}>
      <Segmented value={filter} onChange={onFilter} aria-label="Show">
        {filters.map((f) => (
          <Segment key={f.id} value={f.id}>
            {f.label}
          </Segment>
        ))}
      </Segmented>
      <Input type="text" className={css.search} placeholder="Search the log" value={q} onChange={(e) => onQ(e.target.value)} aria-label="Search the log" />
    </div>
  );
}
