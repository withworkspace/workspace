import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { Menu, MenuItem, MenuSection, MenuText } from "../ui/Menu";
import { Spinner } from "../ui/Spinner";

// StartGroup is the services one worktree can start: its recipes that
// aren't running, by id (what onStart is told) and name. A mission's
// capsules are a group each, titled; one worktree's needs no title.
export interface StartGroup {
  title?: string;
  recipes: { id: string; name: string; title?: string }[];
}

type BelowRunsStartProps = {
  groups: StartGroup[];
  // The one being started, by its id.
  busy?: string;
  onStart: (id: string) => void;
};

// BelowRunsStart is Start, in Below's Runs: a menu of the place's
// service recipes that aren't running, from its ws.repo.toml. With none,
// it isn't there.
export function BelowRunsStart({ groups, busy, onStart }: BelowRunsStartProps) {
  const shown = groups.filter((g) => g.recipes.length > 0);
  if (shown.length === 0) return null;
  const starting = shown.flatMap((g) => g.recipes).find((rc) => rc.id === busy);
  return (
    <Menu
      aria-label="Start a service"
      placement="top end"
      onAction={onStart}
      trigger={
        <Button size="small" disabled={!!busy} title="Start a service from ws.repo.toml">
          {starting ? <Spinner size={11} /> : <Icon name="play" size={11} />} {starting ? `Starting ${starting.name}…` : "Start"}
          {!starting && <Icon name="chevron-down" size={11} />}
        </Button>
      }
    >
      {shown.map((g, i) => (
        <MenuSection key={g.title ?? i} title={shown.length > 1 ? g.title : undefined} aria-label={g.title ?? "Services"}>
          {g.recipes.map((rc) => (
            <MenuItem key={rc.id} id={rc.id} textValue={rc.title || rc.name}>
              <Icon name="play" />
              <MenuText>{rc.title || rc.name}</MenuText>
            </MenuItem>
          ))}
        </MenuSection>
      ))}
    </Menu>
  );
}
