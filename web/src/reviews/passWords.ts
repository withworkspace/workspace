import type { PassFinding, PassMark } from "../api";
import type { IconName } from "../ui/Icon";

// How a review's pass stands, and where what it found is, in words.

export const standingWords = (st: PassMark["standing"]): string => {
  const over = st.overruled ? ` · ${st.overruled} ${st.overruled === 1 ? "blocker" : "blockers"} dismissed` : "";
  switch (st.status) {
    case "none":
      return "Not run";
    case "running":
      return "Running";
    case "failed":
      return "Couldn't run";
    case "nogo":
      return `No go · ${st.blocking} blocking`;
    case "notes":
      return `Go, with notes · ${st.open}${over}`;
    default:
      return `Go${over}`;
  }
};

export const standingIcon = (st: PassMark["standing"]): IconName => (st.status === "nogo" || st.status === "failed" ? "alert" : st.status === "go" || st.status === "notes" ? "check-circle" : "sparkle");

// findingWhere is where a finding is: its file and lines, and on which side.
export const findingWhere = (f: PassFinding) => `${f.path}:${f.line}${f.endLine && f.line && f.endLine > f.line ? `–${f.endLine}` : ""}${f.side === "old" ? " (removed)" : ""}`;
