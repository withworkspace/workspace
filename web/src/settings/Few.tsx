import { Segment, Segmented } from "../ui/Segmented";
import type { ControlProps } from "./controlProps";

// Few is a choice with so few that they're all on show.
export function Few({ s, set }: ControlProps) {
  return (
    <Segmented value={String(s.value)} onChange={(v) => void set(v)} aria-label={s.label}>
      {s.choices?.map((c) => (
        <Segment key={c.value} value={c.value} title={c.note}>
          {c.label ?? c.value}
        </Segment>
      ))}
    </Segmented>
  );
}
