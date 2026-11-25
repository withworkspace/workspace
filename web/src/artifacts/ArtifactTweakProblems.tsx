import { Icon } from "../ui/Icon";
import css from "./ArtifactTweakProblems.module.css";

// ArtifactTweakProblems is what's wrong with how a page declares its
// tweaks, for whoever wrote it: folded, under them.
export function ArtifactTweakProblems({ problems }: { problems: string[] }) {
  return (
    <details className={css.problems}>
      <summary>
        <Icon name="alert" size={12} /> {problems.length === 1 ? "A problem" : `${problems.length} problems`} with what the page declares
      </summary>
      <ul>
        {problems.map((p, i) => (
          <li key={i}>{p}</li>
        ))}
      </ul>
    </details>
  );
}
