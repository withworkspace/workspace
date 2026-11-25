import { Icon } from "../ui/Icon";
import css from "./TalkProposalWarnings.module.css";

// TalkProposalWarnings is what in an artifact proposed won't load in its
// frame (said to the agent too, when it proposed); nothing, when there's
// none.
export function TalkProposalWarnings({ warnings }: { warnings?: string[] }) {
  if (!warnings?.length) return null;
  return (
    <ul className={css.warnings}>
      {warnings.map((w) => (
        <li key={w}>
          <Icon name="alert" size={12} /> {w}
        </li>
      ))}
    </ul>
  );
}
