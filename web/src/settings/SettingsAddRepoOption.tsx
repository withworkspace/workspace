import type { AddableRepo } from "../api";
import { Chip } from "../ui/Chip";
import { ListBoxOption } from "../ui/ListBox";
import { relative } from "../ui/when";
import css from "./SettingsAddRepoOption.module.css";
import { Text } from "../ui/Text";

// SettingsAddRepoOption is a repo on GitHub in the list to add from: its
// name, what it is (private, a fork, archived), when it was last pushed,
// and what it says of itself. One that's here already can't be picked.
export function SettingsAddRepoOption({ repo: r }: { repo: AddableRepo }) {
  const here = !!r.here;
  return (
    <ListBoxOption value={r.name} textValue={r.name} disabled={here} className={css.repo} title={here ? `Already added as ${r.here}` : undefined}>
      <span className={css.line}>
        <Text mono>{r.name}</Text>
        {r.private && <Chip>private</Chip>}
        {r.fork && <Chip>fork</Chip>}
        {r.archived && <Chip tone="quiet">archived</Chip>}
        {here && <Chip tone="done">added{r.here !== r.name ? ` as ${r.here}` : ""}</Chip>}
        {r.pushedAt && <Text dim className={css.pushed}>pushed {relative(r.pushedAt)}</Text>}
      </span>
      {r.description && <Text dim className={css.about}>{r.description}</Text>}
    </ListBoxOption>
  );
}
