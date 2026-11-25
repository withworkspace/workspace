import { Copyable } from "../../ui/Copyable";
import css from "./MissionArtifactPath.module.css";

// MissionArtifactPath is where one of a mission's documents is on disk,
// to copy, at the page's foot.
export function MissionArtifactPath({ path }: { path: string }) {
  return (
    <p className={css.path}>
      <Copyable text={path} title="Copy where it is" />
    </p>
  );
}
