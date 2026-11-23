import { useEffect, useMemo, useState } from "react";
import { type AddableRepo, addRepo, getGitHubOwners, getGitHubRepos } from "../api";
import { useLoaded } from "../load";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { cx } from "../ui/cx";
import { Callout, Skeleton, toast } from "../ui/kit";
import { Dialog } from "../ui/Dialog";
import { Option, Select } from "../ui/Select";
import { relative } from "../ui/time";
import css from "./AddRepo.module.css";
import { GroupPick } from "./Groups";
import { Checkbox } from "../ui/Checkbox";

const ANOTHER = "\u0000another";

// AddRepo picks a repo from GitHub and adds it to the workspace: whose
// (you, or an org you're in, or another named), which of theirs (newest
// pushed first, forks and archived ones hidden unless asked for), its
// name here, and whether it's yours alone or the team's. What GitHub said
// is shown at once, however old, and again once it's been asked: seen
// moves when it has (the github topic, followed by the page).
export function AddRepo({ seen, org, groups, taken, onClose, onAdded }: { seen: number; org?: string; groups: string[]; taken: Set<string>; onClose: () => void; onAdded: () => void }) {
  const owners = useLoaded(() => getGitHubOwners(), []);
  const [picked, setPicked] = useState<string>();
  const [typed, setTyped] = useState("");
  const [another, setAnother] = useState(false);
  const list = owners.value?.owners ?? [];
  const owner = another ? typed.trim() : (picked ?? list.find((o) => o.login.toLowerCase() === org?.toLowerCase())?.login ?? list[0]?.login ?? "");
  const repos = useLoaded(owner ? () => getGitHubRepos(owner) : null, [owner], { keep: true });
  // A look behind finished, or a clone: what's shown is asked for again.
  useEffect(() => {
    if (seen === 0) return;
    owners.reload();
    if (owner) repos.reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seen]);

  const [q, setQ] = useState("");
  const [all, setAll] = useState(false);
  const [chosen, setChosen] = useState<AddableRepo>();
  const shown = useMemo(() => {
    const got = repos.value?.owner.toLowerCase() === owner.toLowerCase() ? repos.value.repos : [];
    const words = q.trim().toLowerCase();
    return got.filter((r) => (all || (!r.fork && !r.archived)) && (!words || `${r.name} ${r.description ?? ""}`.toLowerCase().includes(words)));
  }, [repos.value, owner, q, all]);
  const hidden = (repos.value?.repos ?? []).filter((r) => r.fork || r.archived).length;
  const looking = repos.value?.looking || owners.value?.looking;
  const at = repos.value?.at;

  const look = () => {
    getGitHubOwners(true).catch(() => undefined);
    if (owner) getGitHubRepos(owner, true).then(repos.set, () => undefined);
  };

  return (
    <Dialog title="Add a repo from GitHub" onClose={onClose} wide>
      <div className={css.top}>
        {list.length > 0 ? (
          <Select
            aria-label="Whose repos"
            value={another ? ANOTHER : owner}
            onChange={(v) => {
              setChosen(undefined);
              if (v === ANOTHER) setAnother(true);
              else {
                setAnother(false);
                setPicked(v);
              }
            }}
          >
            {[
              ...list.map((o) => (
                <Option key={o.login} value={o.login} note={o.you ? "you" : undefined}>
                  {o.login}
                </Option>
              )),
              <Option key={ANOTHER} value={ANOTHER}>
                Another owner…
              </Option>,
            ]}
          </Select>
        ) : null}
        {(another || (owners.value?.problem && list.length === 0)) && (
          <input
            className={css.owner}
            placeholder="owner on GitHub"
            aria-label="Owner on GitHub"
            value={typed}
            onChange={(e) => {
              setAnother(true);
              setTyped(e.target.value);
              setChosen(undefined);
            }}
            autoFocus
          />
        )}
        <input className={css.filter} placeholder="Filter…" aria-label="Filter the repos" value={q} onChange={(e) => setQ(e.target.value)} />
        <span className={cx("dim", css.when)}>
          {looking ? "asking GitHub…" : at ? `as of ${relative(at)}` : ""}
          {!looking && (
            <Button variant="link" onClick={look}>
              Look again
            </Button>
          )}
        </span>
      </div>

      {owners.value?.problem && list.length === 0 && (
        <Callout tone="danger" title="GitHub couldn't say whose repos there are">
          {owners.value.problem}. Name an owner to look at theirs.
        </Callout>
      )}
      {repos.value?.problem && (
        <Callout tone="danger" title={`GitHub couldn't list ${owner}'s repos`}>
          {repos.value.problem}
          {/SAML|SSO/i.test(repos.value.problem) && (
            <>
              {" "}
              — authorise gh's token for {owner}'s single sign-on, at{" "}
              <a href={`https://github.com/orgs/${owner}/sso`} target="_blank" rel="noreferrer">
                github.com/orgs/{owner}/sso
              </a>
              , then run <code>gh auth refresh</code>.
            </>
          )}
        </Callout>
      )}

      <div className={css.list} role="listbox" aria-label={`${owner}'s repos`}>
        {!repos.value && (owner ? <Skeleton rows={5} /> : null)}
        {repos.value && shown.length === 0 && !looking && <p className={cx("dim", css.none)}>{q ? "None of these match." : "Nothing to add here."}</p>}
        {repos.value && shown.length === 0 && looking && <Skeleton rows={5} />}
        {shown.map((r) => {
          const here = !!r.here;
          const isChosen = chosen?.name === r.name;
          return (
            <button
              key={r.name}
              type="button"
              role="option"
              aria-selected={isChosen}
              disabled={here}
              className={cx(css.repo, isChosen && css.chosen)}
              onClick={() => setChosen(r)}
              title={here ? `Here already, as ${r.here}` : undefined}
            >
              <span className={css.repoLine}>
                <span className="mono">{r.name}</span>
                {r.private && <Chip>private</Chip>}
                {r.fork && <Chip>fork</Chip>}
                {r.archived && <Chip tone="quiet">archived</Chip>}
                {here && <Chip tone="done">here{r.here !== r.name ? ` as ${r.here}` : ""}</Chip>}
                {r.pushedAt && <span className={cx("dim", css.pushed)}>pushed {relative(r.pushedAt)}</span>}
              </span>
              {r.description && <span className={cx("dim", css.about)}>{r.description}</span>}
            </button>
          );
        })}
      </div>
      {hidden > 0 && (
        <Checkbox className={cx("dim", css.all)} checked={all} onChange={setAll}>
          Show forks and archived repos ({hidden})
        </Checkbox>
      )}

      {chosen && <Adding key={`${owner}/${chosen.name}`} owner={owner} repo={chosen} taken={taken} groups={groups} onCancel={() => setChosen(undefined)} onAdded={onAdded} />}
    </Dialog>
  );
}

// Adding is the repo chosen, about to be added: its name here (its name
// on GitHub, unless that's taken here), and whose it is.
function Adding({ owner, repo, taken, groups, onCancel, onAdded }: { owner: string; repo: AddableRepo; taken: Set<string>; groups: string[]; onCancel: () => void; onAdded: () => void }) {
  const [as, setAs] = useState(() => (taken.has(repo.name) ? `${owner}-${repo.name}` : repo.name));
  const [group, setGroup] = useState("");
  const [team, setTeam] = useState(false);
  const [busy, setBusy] = useState(false);
  const name = as.trim();
  const clash = taken.has(name);
  const bad = !/^[A-Za-z0-9_-][A-Za-z0-9._-]*$/.test(name);
  const add = () => {
    setBusy(true);
    addRepo({ owner, name: repo.name, as: name !== repo.name ? name : undefined, group: group || undefined, team })
      .then((r) => {
        toast(r.message);
        onAdded();
      })
      .catch((e: Error) => {
        toast(e.message, { tone: "danger", ms: 8000 });
        setBusy(false);
      });
  };
  return (
    <form
      className={css.adding}
      onSubmit={(e) => {
        e.preventDefault();
        if (!clash && !bad && !busy) add();
      }}
    >
      <div className={css.field}>
        <label htmlFor="add-repo-as">Its name here</label>
        <input id="add-repo-as" className="mono" value={as} onChange={(e) => setAs(e.target.value)} aria-invalid={clash || bad} />
        <span className={cx("dim", css.hint)}>
          {clash ? `There's a ${name} here already.` : bad ? "Letters, digits, '.', '_' and '-'." : name !== repo.name ? `It stays ${owner}/${repo.name} on GitHub.` : "Its folder under repos/, and what commands call it."}
        </span>
      </div>
      <div className={css.field}>
        <span className={css.label}>Its group</span>
        <span>
          <GroupPick value={group} groups={groups} none label="Its group" onPick={setGroup} />
        </span>
      </div>
      <fieldset className={css.field}>
        <legend>Whose</legend>
        <span className={css.whose}>
          <label>
            <input type="radio" name="add-repo-whose" checked={!team} onChange={() => setTeam(false)} /> Just me <span className="dim">— in ws.local.toml</span>
          </label>
          <label>
            <input type="radio" name="add-repo-whose" checked={team} onChange={() => setTeam(true)} /> The team <span className="dim">— in ws.toml, committed and pushed</span>
          </label>
        </span>
      </fieldset>
      <div className={css.buttons}>
        <Button variant="ghost" type="button" onClick={onCancel} disabled={busy}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={clash || bad || busy}>
          {busy ? "Adding…" : team ? `Add ${owner}/${repo.name} for the team` : `Add ${owner}/${repo.name}`}
        </Button>
      </div>
    </form>
  );
}
