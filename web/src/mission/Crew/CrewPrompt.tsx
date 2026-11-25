import { useState } from "react";
import { answerCrew, type Crew, type CrewAnswer, type CrewPrompt as Screen, getCrewPrompt, type TranscriptEntry } from "../../api";
import { useLoaded } from "../../load";
import { Button } from "../../ui/Button";
import { confirm } from "../../ui/Confirm";
import { EditDiff } from "../../ui/EditDiff";
import { Icon } from "../../ui/Icon";
import { MoreLink } from "../../ui/MoreLink";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";
import { MissionCrewPrompt, MissionCrewPromptActions, MissionCrewPromptAnswer, MissionCrewPromptFile, MissionCrewPromptInput, MissionCrewPromptOptions, MissionCrewPromptPath } from "./MissionCrewPrompt";
import { screenContext, shortPath, wantsTo } from "./words";

type CrewPromptProps = {
  slug: string;
  crew: Crew;
  pending?: TranscriptEntry;
  fileLink?: (path: string) => string | undefined;
  onTerminal?: () => void;
  compact?: boolean;
};

// CrewPrompt is what crew are stopped on, answered from here: a permission
// (the step it wants to take, and allow or deny) or a question (its
// options, or your own words). Answering types the keys a person would,
// once the server's read the screen and seen the same prompt; if it
// hasn't, the terminal's the place.
export function CrewPrompt({ slug, crew, pending, fileLink, onTerminal, compact }: CrewPromptProps) {
  const [busy, setBusy] = useState(false);
  const [own, setOwn] = useState("");
  // What it's asking about, read off its screen, when the transcript
  // hasn't the step yet (a harness can write it late).
  const { value: screen } = useLoaded<Screen | undefined>(
    crew.activity === "permission" && !pending ? () => getCrewPrompt(slug, crew.id).catch(() => undefined) : null,
    [slug, crew.id, crew.activity, crew.notice, pending],
  );
  const about = screenContext(screen);
  const answer = async (a: CrewAnswer, ask?: { title: string; body: string; confirm: string }) => {
    if (ask && !(await confirm({ ...ask }))) return;
    setBusy(true);
    answerCrew(slug, crew.id, a)
      .then(
        (r) => toast(r.message, { tone: "done" }),
        (e: Error) =>
          toast(humanError(e.message), {
            tone: "danger",
            ms: 8000,
            action: onTerminal ? { label: "Open the terminal", run: onTerminal } : undefined,
          }),
      )
      .finally(() => setBusy(false));
  };
  // Crew over ACP have no terminal: the card is the only way to answer.
  const terminal = crew.structured ? null : onTerminal ? (
    <MoreLink onClick={onTerminal} keys="t">
      Answer in the terminal
    </MoreLink>
  ) : (
    <MoreLink to={`/missions/${slug}/crew/${crew.id}?view=terminal`}>
      Answer in the terminal
    </MoreLink>
  );
  if (crew.activity === "permission") {
    const link = pending?.edit && fileLink?.(pending.edit.path);
    return (
      <MissionCrewPrompt
        kind="permission"
        compact={compact}
        aria-label={`${crew.id} needs your permission`}
        head={
          <>
            <strong>{crew.id}</strong> wants to{" "}
            {wantsTo(pending, screen, about)}
            {pending?.edit && <MissionCrewPromptPath>{shortPath(pending.edit.path)}</MissionCrewPromptPath>}
          </>
        }
      >
        {pending && !pending.edit && pending.input && <MissionCrewPromptInput>{pending.input}</MissionCrewPromptInput>}
        {pending?.edit && !compact && <EditDiff edit={pending.edit} max={10} />}
        {link && !compact && <MissionCrewPromptFile to={link} />}
        {!pending && !!about?.length && <MissionCrewPromptInput>{about.join("\n")}</MissionCrewPromptInput>}
        {!pending && !about?.length && crew.notice && <div className="dim">{crew.notice}</div>}
        <MissionCrewPromptActions more={terminal}>
          <Button variant="primary" disabled={busy} onClick={() => answer({ permission: "allow" })}>
            <Icon name="check" size={13} /> Allow once
          </Button>
          <Button
            disabled={busy}
            onClick={() =>
              answer(
                { permission: "always" },
                {
                  title: "Allow it from now on?",
                  body: `${crew.id} won't ask again for this kind of thing, for the rest of its session. Its harness decides what "this kind" covers.`,
                  confirm: "Allow from now on",
                },
              )
            }
          >
            Always allow…
          </Button>
          <Button variant="ghost" disabled={busy} onClick={() => answer({ permission: "deny" })}>
            Deny
          </Button>
        </MissionCrewPromptActions>
      </MissionCrewPrompt>
    );
  }
  // A question, with its own options, or your words.
  const q = pending?.questions?.[0];
  const options = crew.options?.length ? crew.options : (q?.options ?? []);
  return (
    <MissionCrewPrompt
      kind="question"
      compact={compact}
      aria-label={`${crew.id} asks you`}
      head={
        <>
          <strong>{crew.id}</strong> asks: {crew.notice || q?.question || "a question"}
        </>
      }
    >
      {options.length > 0 && (
        <MissionCrewPromptOptions>
          {options.map((o) => (
            <Button key={o} disabled={busy} onClick={() => answer({ option: o })}>
              {o}
            </Button>
          ))}
        </MissionCrewPromptOptions>
      )}
      <MissionCrewPromptAnswer value={own} onChange={setOwn} busy={busy} onSubmit={() => own.trim() && answer({ text: own })} />
      <MissionCrewPromptActions more={terminal} />
    </MissionCrewPrompt>
  );
}
