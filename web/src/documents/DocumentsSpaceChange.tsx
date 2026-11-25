import type { SpaceDocChange } from "../api";
import { cx } from "../ui/cx";
import { TLink } from "../ui/TLink";
import { mark } from "./space";
import css from "./DocumentsSpaceChange.module.css";
import { Text } from "../ui/Text";

// DocumentsSpaceChange is a document a capsule changes: what's done to it
// (+, ~, −) and its name, to it as it is there (to), unless it's removed.
export function DocumentsSpaceChange({ how, name, path, to }: { how: SpaceDocChange["how"]; name: string; path: string; to?: string }) {
  return (
    <span className={cx(css.change, how === "added" && css.added, how === "removed" && css.removed)} title={`${path}: ${how}`}>
      <Text mono className={css.mark}>{mark(how)}</Text> {to ? <TLink to={to}>{name}</TLink> : <Text dim>{name}</Text>}
    </span>
  );
}
