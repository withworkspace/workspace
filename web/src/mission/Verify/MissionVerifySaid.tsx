import { Inline } from "../../ui/Inline";
import css from "./MissionVerifySaid.module.css";

// MissionVerifySaid is what you said wasn't right when you sent a bundle
// back, a line each, with the check it's of.
export function MissionVerifySaid({ notes }: { notes: { check?: number; text: string }[] }) {
  return (
    <ul className={css.said}>
      {notes.map((n, i) => (
        <li key={i}>
          {n.check ? `Check ${n.check}: ` : ""}
          <Inline text={n.text} />
        </li>
      ))}
    </ul>
  );
}
