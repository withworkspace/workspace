import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { GROUND, getWorktrees, type Repo, type Worktree } from "../../api";
import { useCommands } from "../../keymap/useCommands";
import { switchKeys } from "../keys";
import { useLoaded } from "../../load";
import { useWorkspace } from "../../workspace/store";
import { FilesSwitch } from "./FilesSwitch";
import { FilesSwitchItem, FilesSwitchMenu } from "./FilesSwitchMenu";

// WorktreeSwitch is the title: the repo and which of its worktrees this
// is, the ground or a capsule, and (open) the others, to switch to. The
// file open stays open, if it's in the other.
export function WorktreeSwitch({ repo, current, open, setOpen }: { repo: Repo; current: string; open: boolean; setOpen: (v: boolean) => void }) {
  const capsulesSeen = useWorkspace((w) => w.capsulesSeen);
  const missions = useWorkspace((w) => w.missions);
  const navigate = useNavigate();
  const location = useLocation();
  // Answered at once, with what git hasn't said yet unread: read again when
  // it has (the workspace's capsulesSeen moves).
  const { value: wts } = useLoaded<Worktree[]>(() => getWorktrees(repo.name).catch(() => []), [repo.name, capsulesSeen], { keep: true });
  // Until their branches are read, what the repo's listing says.
  const list: Worktree[] = wts ?? [{ name: GROUND, ground: true, branch: "" }, ...repo.capsules.map((c) => ({ name: c, branch: "" }))];
  const [sel, setSel] = useState(0);
  useEffect(() => {
    if (open) setSel(Math.max(0, list.findIndex((w) => w.name === current)));
    // Only as it opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
  const go = (w: Worktree) => {
    setOpen(false);
    if (w.name !== current) navigate(`/files/${encodeURIComponent(repo.name)}${w.ground ? "" : `/${encodeURIComponent(w.name)}`}${location.search}`);
  };
  const box = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => !box.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, [open, setOpen]);
  useCommands(switchKeys, { open: !open && (() => setOpen(true)) });
  // While it's open, its keys are the only ones.
  useCommands(
    switchKeys,
    open
      ? {
          close: () => setOpen(false),
          next: () => setSel((i) => Math.min(i + 1, list.length - 1)),
          prev: () => setSel((i) => Math.max(i - 1, 0)),
          go: () => list[sel] && go(list[sel]),
          open: () => setOpen(false),
        }
      : {},
    { modal: open },
  );
  const here = list.find((w) => w.name === current);
  const missionOf = (name: string) => missions.find((m) => m.capsules.some((c) => c.repo === repo.name && c.name === name))?.title;
  const sub = (w: Worktree) => {
    if (w.ground) return w.branch ? `${w.branch}, the default branch` : "the default branch";
    return missionOf(w.name) || (w.branch && w.branch !== w.name ? w.branch : "a capsule");
  };
  return (
    <FilesSwitch
      ref={box}
      repo={repo.label}
      color={repo.color}
      name={current}
      ground={current === GROUND}
      dirty={here?.dirty}
      open={open}
      onToggle={() => setOpen(!open)}
      menu={
        <FilesSwitchMenu label={`${repo.label}'s ground and capsules`} none={wts && wts.length <= 1 && <>No capsules yet: run ws lift {repo.name} &lt;name&gt;</>}>
          {list.map((w, i) => (
            <FilesSwitchItem
              key={w.name}
              name={w.name}
              ground={w.ground}
              sub={sub(w)}
              ahead={w.ahead}
              behind={w.behind}
              dirty={w.dirty}
              current={w.name === current}
              on={i === sel}
              onHover={() => i !== sel && setSel(i)}
              onPick={() => go(w)}
            />
          ))}
        </FilesSwitchMenu>
      }
    />
  );
}
