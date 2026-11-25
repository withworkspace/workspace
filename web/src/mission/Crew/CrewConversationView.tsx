import type { Ref } from "react";
import type { Crew, Transcript } from "../../api";
import { Markdown } from "../../markdown/LazyMarkdown";
import { Button } from "../../ui/Button";
import { Calls } from "../../ui/Calls";
import { Empty } from "../../ui/Empty";
import { ErrorCard } from "../../ui/ErrorCard";
import { Loading } from "../../ui/Loading";
import { Turn, TurnAvatar, TurnHead, TurnSaid } from "../../ui/Turn";
import { CrewBadge } from "../CrewBadge";
import { CrewPlan } from "./CrewPlan";
import { CrewPrompt } from "./CrewPrompt";
import { CrewTell } from "./CrewTell";
import { latestPlan, pendingCall, toCall, transcriptGroups } from "./groups";
import { MissionCrewBriefed } from "./MissionCrewBriefed";
import { MissionCrewCalls } from "./MissionCrewCalls";
import { MissionCrewConversation } from "./MissionCrewConversation";
import { MissionCrewDock } from "./MissionCrewDock";
import { MissionCrewEarlier } from "./MissionCrewEarlier";
import { MissionCrewNotice } from "./MissionCrewNotice";
import { MissionCrewTranscript } from "./MissionCrewTranscript";
import { MissionCrewWorking } from "./MissionCrewWorking";
import { noticeWords, workingOn } from "./words";

type CrewConversationViewProps = {
  slug: string;
  id: string;
  crew?: Crew;
  live: boolean;
  // Its transcript, as read so far, and what went wrong reading it.
  t?: Transcript;
  error?: string;
  reload?: () => void;
  // Where the workspace is, and where a file crew edited can be read: for
  // its steps.
  root: string;
  fileLink?: (path: string) => string | undefined;
  // There's more below what's read: the way down to it.
  onNewer?: () => void;
  onTerminal?: () => void;
  // The box to write to crew, for whoever gives it the keys.
  field?: Ref<HTMLTextAreaElement>;
};

// CrewConversationView says which part each thing of crew's transcript is
// drawn as: ws's brief to it, what it was sent and said (Turns), its steps
// (Calls), a harness's notice, and its plan on top; with what it's
// stopped on and the box to tell it what's next docked under them.
export function CrewConversationView({ slug, id, crew, live, t, error, reload, root, fileLink, onNewer, onTerminal, field }: CrewConversationViewProps) {
  if (error && !t) return <ErrorCard title="Couldn't read the transcript" error={error} retry={reload} />;
  if (!t) return <Loading what="Reading the transcript…" />;
  if (!t.available && !live)
    return (
      <Empty icon="message" title="No transcript">
        {t.why}
      </Empty>
    );
  const groups = transcriptGroups(t);
  const plan = latestPlan(t);
  const pending = pendingCall(t);
  const asking = live && (crew?.activity === "permission" || crew?.activity === "question");
  const working = live && crew?.activity === "working";
  const now = working && !asking && pending ? workingOn(pending) : undefined;
  return (
    <MissionCrewConversation
      plan={plan && <CrewPlan todos={plan} live={live} />}
      note={
        !t.available && (
          <Empty icon="terminal" title="No transcript to show" action={onTerminal && <Button onClick={onTerminal} keys="t">Watch its terminal</Button>}>
            {t.why} You can still send it a message below.
          </Empty>
        )
      }
      dock={
        <MissionCrewDock onNewer={onNewer}>
          {asking && crew && <CrewPrompt slug={slug} crew={crew} pending={pending} fileLink={fileLink} onTerminal={onTerminal} />}
          <CrewTell slug={slug} id={id} crew={crew} live={live} working={working} field={field} />
        </MissionCrewDock>
      }
    >
      <MissionCrewTranscript>
        {!!t.earlier && <MissionCrewEarlier count={t.earlier} />}
        {groups.map((g, i) => {
          const latest = i === groups.length - 1;
          switch (g.kind) {
            case "brief":
              return <MissionCrewBriefed key={g.at} id={id} text={g.text} time={g.time} />;
            case "notice":
              return <MissionCrewNotice key={g.at}>{noticeWords(g.text)}</MissionCrewNotice>;
            case "calls":
              return (
                <MissionCrewCalls key={g.at}>
                  <Calls calls={g.calls.map(toCall)} open={live && latest} live={working} root={root} fileLink={fileLink} />
                </MissionCrewCalls>
              );
            default:
              return (
                <Turn
                  key={g.at}
                  by={g.kind === "user" ? "you" : "agent"}
                  head={<TurnHead avatar={g.kind === "user" ? <TurnAvatar by="you" /> : <CrewBadge id={id} />} who={g.kind === "user" ? "Sent" : undefined} at={g.time} />}
                >
                  <TurnSaid>
                    <Markdown text={g.text} />
                  </TurnSaid>
                </Turn>
              );
          }
        })}
        {!t.entries.length && t.available && (
          <Empty icon="message" title="Nothing yet">
            What {id} is sent, says and does shows here as it happens.
          </Empty>
        )}
        {working && !asking && (
          <MissionCrewWorking>
            {now ? (
              <>
                {now.verb} <code>{now.what}</code>
              </>
            ) : (
              "Working"
            )}
          </MissionCrewWorking>
        )}
      </MissionCrewTranscript>
    </MissionCrewConversation>
  );
}
