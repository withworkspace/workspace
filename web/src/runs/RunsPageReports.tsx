import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import css from "./RunsPageReports.module.css";

export interface RunsPageReport {
  name: string;
  // title: where it is, and whether it's kept.
  title: string;
  // unwritten: its command didn't write it, said in the waiting tone.
  unwritten: boolean;
}

// RunsPageReports is the reports kept with a task, a chip each, by name.
export function RunsPageReports({ reports }: { reports: RunsPageReport[] }) {
  return (
    <span className={css.reports}>
      {reports.map((rp) => (
        <Chip key={rp.name} tone={rp.unwritten ? "waiting" : "quiet"} className={css.report} title={rp.title}>
          <Icon name="file" size={10} /> {rp.name}
          {rp.unwritten ? " not written" : ""}
        </Chip>
      ))}
    </span>
  );
}
