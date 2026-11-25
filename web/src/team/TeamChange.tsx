import type { TeamEvent, TeamItem } from "../api";
import { prHref } from "../reviews/Reviews";
import { Button } from "../ui/Button";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import type { StatusInfo } from "../ui/Status";
import { humanError } from "../ui/text";
import { TeamEventLine, TeamEvents, TeamEventSubject } from "./TeamEvents";
import { TeamGist } from "./TeamGist";
import { TeamKnow } from "./TeamKnow";
import { TeamNear } from "./TeamNear";
import { TeamPulls } from "./TeamPulls";
import { TeamRepoChip } from "./TeamRepoChip";
import { TeamRow } from "./TeamRow";
import { TeamState } from "./TeamState";
import { eventIcon, eventWords, stateInfo } from "./words";

interface TeamChangeProps {
  id: string;
  item: TeamItem;
  names: string[];
  repoOf: (repo: string) => { label: string; color?: string };
  selected: boolean;
  open: boolean;
  // fresh: something happened to it since you last looked.
  fresh: boolean;
  // dated: it's not under its day, so when is said with the day.
  dated: boolean;
  // know: this is the row what's worth knowing of its change is said on.
  know: boolean;
  who: (e: TeamEvent) => string | undefined;
  onHover: () => void;
  onToggle: () => void;
  onOpen: () => void;
  onWrite: () => void;
}

// TeamChange is a change of the Team page's as a TeamRow: where it
// stands, where it was made, what it does and what's worth knowing of it,
// and, opened, its pull requests and what happened to it.
export function TeamChange({ id, item: it, names, repoOf, selected, open, fresh, dated, know, who, onHover, onToggle, onOpen, onWrite }: TeamChangeProps) {
  const status: StatusInfo = it.kind === "commits" ? { icon: "commit", tone: "done", words: "Landed" } : stateInfo[it.state];
  const here = it.kind === "pr" && (it.state === "in-review" || it.state === "in-progress");
  const repo = repoOf(it.repo);
  // One change in several repos: how many of its pull requests have landed.
  const several = it.prs && it.prs.length > 1 ? it.prs : undefined;
  const landed = several?.filter((p) => p.state === "landed").length ?? 0;
  return (
    <TeamRow
      id={id}
      status={status}
      name={it.name}
      beside={
        <>
          {several ? (
            <TeamRepoChip colors={(it.repos ?? []).slice(0, 4).map((r) => repoOf(r).color)} label={`${it.repos?.length} repos`} title={(it.repos ?? []).map((r) => repoOf(r).label).join(", ")} />
          ) : (
            <TeamRepoChip colors={[repo.color]} label={repo.label} pr={it.pr} title={it.repo} />
          )}
          {it.touches?.map((t) => (
            <Chip tone="progress" key={t.name} title={`It changes how things are done:\n${t.files.join("\n")}`}>
              {t.title}
            </Chip>
          ))}
          <TeamState tone={status.tone}>
            {status.words}
            {several && landed > 0 && landed < several.length ? `, ${landed} of ${several.length} landed` : ""}
          </TeamState>
        </>
      }
      under={
        <>
          <TeamGist text={it.gist} reading={it.narrative === "writing"} />
          {know && it.know?.map((k) => <TeamKnow key={`${k.kind}:${k.file}:${k.text}`} know={k} repo={it.repo} />)}
          {it.near && <TeamNear near={it.near} />}
        </>
      }
      words={it.words}
      people={names.join(", ")}
      additions={it.additions}
      deletions={it.deletions}
      at={it.at}
      dated={dated}
      selected={selected}
      open={open}
      fresh={fresh}
      onHover={onHover}
      onToggle={onToggle}
    >
      <TeamEvents
        pulls={
          several && (
            <TeamPulls
              pulls={several.map((p) => {
                const open = p.state === "in-review" || p.state === "in-progress";
                return { key: `${p.repo}#${p.number}`, status: stateInfo[p.state], repo: repoOf(p.repo).label, color: repoOf(p.repo).color, number: p.number, to: open ? prHref(p.repo, p.number) : undefined, href: p.url };
              })}
            />
          )
        }
        failed={it.narrative === "failed" && `What it does couldn't be written: ${humanError(it.gistError ?? "")}`}
        actions={
          <>
            {!several && (
              <Button variant="ghost" size="small" onClick={onOpen} title="Open it" keys="o">
                <Icon name={here ? "pr" : "external"} size={13} /> {here ? "Review it here" : "Open on GitHub"} <Keys k="o" />
              </Button>
            )}
            {(it.narrative === "unwritten" || it.narrative === "failed") && (
              <Button variant="ghost" size="small" onClick={onWrite} title="An agent reads the change and writes what it does, and what's worth knowing of it" keys="w">
                <Icon name="sparkle" size={13} /> {it.narrative === "failed" ? "Try again" : it.gist ? "Read it again" : "Read the change"} <Keys k="w" />
              </Button>
            )}
          </>
        }
      >
        {it.events.map((e) => (
          <TeamEventLine key={e.id} at={e.at} icon={eventIcon(e)} commit={e.commit} paths={e.paths} files={e.files}>
            {e.kind !== "closed" && <strong>{who(e) ?? e.login ?? e.name ?? "Someone"} </strong>}
            {eventWords(e)}
            {several && <span className="dim"> in {repoOf(e.repo).label}</span>}
            {e.kind === "landed" && <TeamEventSubject>{e.title}</TeamEventSubject>}
          </TeamEventLine>
        ))}
      </TeamEvents>
    </TeamRow>
  );
}
