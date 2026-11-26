import { Copyable } from "../../ui/Copyable";
import css from "./MissionVerifyLines.module.css";

// MissionVerifyLines is where in the capsule a check is done: each place
// (a path and its lines) to copy.
export function MissionVerifyLines({ lines }: { lines: string[] }) {
  return (
    <p className={css.lines}>
      Location:{" "}
      {lines.map((l, i) => (
        <span key={l}>
          {i > 0 && ", "}
          <Copyable text={l} title="Copy location" />
        </span>
      ))}
    </p>
  );
}
