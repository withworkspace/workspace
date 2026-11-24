import { useEffect, useRef, useState } from "react";
import { groupRepo, type Repo, type RepoGroup, showGroup } from "../api";
import { Button } from "../ui/Button";
import { cx } from "../ui/cx";
import { toast } from "../ui/toast";
import { Option, Select } from "../ui/Select";
import css from "./Groups.module.css";
import { Segment, Segmented } from "../ui/Segmented";
import { Input } from "../ui/Input";

const NONE = "\u0000none";
const NEW = "\u0000new";

// GroupPick is which group a repo's in, picked: one the workspace has, a
// new one named, or none, where it can be in none (a repo of the team's
// in a group of the team's can't be, for you alone). The team's group is
// said as the team's. Groups are offered in the order given, by their
// labels (display names) where they have them.
export function GroupPick({ value, groups, labels, team, none, label, onPick, disabled }: { value: string; groups: string[]; labels?: Map<string, string>; team?: string; none: boolean; label: string; onPick: (group: string) => void; disabled?: boolean }) {
  const [naming, setNaming] = useState(false);
  const [name, setName] = useState("");
  const field = useRef<HTMLInputElement>(null);
  // It comes in place of the list it was picked from, which holds the
  // keys until it's closed: focused once it has.
  useEffect(() => {
    if (!naming) return;
    const frame = requestAnimationFrame(() => field.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [naming]);
  if (naming) {
    const done = (keep: boolean) => {
      setNaming(false);
      const n = name.trim();
      setName("");
      if (keep && n && n !== value) onPick(n);
    };
    const bad = /[\s,]/.test(name.trim());
    return (
      <Input
        ref={field}
        className={cx("mono", css.naming)}
        aria-label={`${label}: a new group`}
        placeholder="new group"
        value={name}
        aria-invalid={bad}
        title={bad ? "No spaces or commas" : undefined}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !bad) done(true);
          if (e.key === "Escape") {
            e.stopPropagation();
            done(false);
          }
        }}
        onBlur={() => done(!bad)}
      />
    );
  }
  const all = [...new Set([...groups, ...[value, team].filter((g): g is string => !!g && !groups.includes(g)).sort()])];
  return (
    <Select
      aria-label={label}
      value={value || NONE}
      disabled={disabled}
      onChange={(v) => {
        if (v === NEW) setNaming(true);
        else if (v === NONE) onPick("");
        else if (v !== value) onPick(v);
      }}
    >
      {[
        ...(none ? [<Option key={NONE} value={NONE}>No group</Option>] : []),
        ...all.map((g) => (
          <Option key={g} value={g} note={g === team ? "the team's" : undefined}>
            {labels?.get(g) ?? g}
          </Option>
        )),
        <Option key={NEW} value={NEW}>
          New group…
        </Option>,
      ]}
    </Select>
  );
}

// RepoGroupCell is a repo's group in the repos table, changed for you in
// ws.local.toml: a repo of yours in any or none, one of the team's in any,
// or back in the team's.
export function RepoGroupCell({ repo, groups, labels, done }: { repo: Repo; groups: string[]; labels?: Map<string, string>; done: () => void }) {
  const [busy, setBusy] = useState(false);
  const team = repo.yours ? undefined : repo.groupYours ? repo.teamGroup || undefined : repo.group || undefined;
  const pick = (group: string) => {
    setBusy(true);
    groupRepo(repo.name, group)
      .then((r) => {
        toast(r.message);
        return done();
      })
      .catch((e: Error) => toast(e.message, { tone: "danger", ms: 8000 }))
      .finally(() => setBusy(false));
  };
  return (
    <span className={css.cell}>
      <GroupPick value={repo.group ?? ""} groups={groups} labels={labels} team={team} none={repo.yours || !team} label={`${repo.name}'s group`} onPick={pick} disabled={busy} />
      {!repo.yours && repo.groupYours && (
        <span className={cx("dim", css.over)} title={repo.teamGroup ? `The team has it in ${repo.teamGroup}` : "The team has it in no group"}>
          yours
        </span>
      )}
    </span>
  );
}

// Groups are the workspace's groups, each shown or hidden for you: hidden
// ones are left out of what covers every repo (status, rg, debrief, sync),
// and their repos are cloned when they're first needed. What the team says
// is beside it, and Reset goes back to it.
export function Groups({ groups, repos, done }: { groups: RepoGroup[]; repos: Repo[]; done: () => void }) {
  const [busy, setBusy] = useState<string>();
  const label = new Map(repos.map((r) => [r.name, r.label]));
  const set = (g: string, hidden: boolean | null) => {
    setBusy(g);
    showGroup(g, hidden)
      .then((r) => {
        toast(r.message);
        return done();
      })
      .catch((e: Error) => toast(e.message, { tone: "danger", ms: 8000 }))
      .finally(() => setBusy(undefined));
  };
  return (
    <div className={css.groups}>
      <h3 className={css.head}>Groups</h3>
      {groups.length === 0 ? (
        <p className="settings-about">None yet: put a repo in one from its row. A hidden group is left out of what covers every repo, its repos cloned when they're first needed.</p>
      ) : (
        <>
          <p className="settings-about">A hidden group is left out of what covers every repo (status, rg, debrief, sync), its repos cloned when they're first needed. Shown or hidden is yours, in ws.local.toml.</p>
          <table className="settings-table">
            <thead>
              <tr>
                <th>Group</th>
                <th>Repos</th>
                <th>For you</th>
                <th aria-label="The team's" />
              </tr>
            </thead>
            <tbody>
              {groups.map((g) => (
                <tr key={g.name}>
                  <td>
                    {g.label && g.label !== g.name ? (
                      <span className={css.named}>
                        {g.label} <span className="mono dim">{g.name}</span>
                      </span>
                    ) : (
                      <span className="mono">{g.name}</span>
                    )}
                  </td>
                  <td className="dim">{g.repos.length ? g.repos.map((r) => label.get(r) ?? r).join(", ") : "—"}</td>
                  <td>
                    <Segmented value={g.hidden ? "hidden" : "shown"} onChange={(v) => set(g.name, v === "hidden")} disabled={busy === g.name} aria-label={`${g.name}: shown or hidden`}>
                      <Segment value="shown">Shown</Segment>
                      <Segment value="hidden">Hidden</Segment>
                    </Segmented>
                  </td>
                  <td className={cx("dim", css.team)}>
                    {g.chosen ? (
                      <>
                        the team's: {g.teamHidden ? "hidden" : "shown"}
                        <Button variant="link" className={css.reset} disabled={busy === g.name} onClick={() => set(g.name, null)}>
                          Reset
                        </Button>
                      </>
                    ) : (
                      "as the team has it"
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  );
}
