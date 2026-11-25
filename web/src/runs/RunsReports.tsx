import type { RunDetail } from "../api";
import { RunsPageReports } from "./RunsPageReports";

// RunsReports is the reports kept with a task, by name: one its command
// didn't write says so.
export function RunsReports({ r }: { r: RunDetail }) {
  if (!r.reports?.length) return null;
  const running = r.result === "running";
  return (
    <RunsPageReports
      reports={r.reports.map((rp) => ({
        name: rp.name,
        unwritten: rp.files === 0 && !running,
        title: rp.files > 0 ? `${rp.path}${rp.format ? `, ${rp.format}` : ""}: kept with the run` : running ? `${rp.path}: kept once it ends` : `Its command didn't write ${rp.path}`,
      }))}
    />
  );
}
