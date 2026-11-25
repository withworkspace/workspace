import { type Artifact, getArtifact, type MissionDetail } from "../../api";
import { useLoaded } from "../../load";
import { Markdown } from "../../markdown/LazyMarkdown";
import { Button } from "../../ui/Button";
import { ErrorCard } from "../../ui/ErrorCard";
import { Icon } from "../../ui/Icon";
import { Keys } from "../../ui/Keys";
import { Skeleton } from "../../ui/Skeleton";
import { Time } from "../../ui/Time";
import { MissionOutputState } from "../MissionOutputState";
import { MissionOutputThreads } from "../MissionOutputThreads";
import { CrewBadge } from "../CrewBadge";
import { artifactTitle, isArtifact, kindTitle } from "../outputs";
import type { Home } from "../workRows";
import { MissionWorkBody } from "./MissionWorkBody";
import { MissionWorkEmpty } from "./MissionWorkEmpty";
import { MissionWorkHead } from "./MissionWorkHead";
import { MissionWorkProse } from "./MissionWorkProse";

// The open output: what it is and where it came from, and its text to
// read. Its own page (⏎) is where it's commented on and edited.
export function WorkOutputPane({ d, a, home, label, select, open }: { d: MissionDetail; a: Artifact; home: Home; label: (repo: string) => string; select: (key: string) => void; open: (a: Artifact) => void }) {
  const reads = a.readable && !isArtifact(a);
  const { value: doc, error } = useLoaded(reads ? () => getArtifact(d.slug, a.ref) : null, [d.slug, a.ref, reads], { keep: true });
  const { title, desc } = artifactTitle(a);
  const handoff = a.kind === "handoff";
  // A handoff's text begins with its title and who left it where, which
  // the pane's head says.
  const text = doc?.text.replace(/^# .*\n+/, "").replace(/^Handoff from crew .*\n+/, "") ?? "";
  return (
    <>
      <MissionWorkHead
        kind={
          <>
            {handoff ? "Handoff" : kindTitle(a.kind).replace(/s$/, "")}
            {home.step && ` · step ${home.step.n}, ${home.step.title}`} · <Time iso={a.added} />
          </>
        }
        actions={
          <Button size="small" onClick={() => open(a)} title={reads ? "Open its page, to comment on it or edit it (⏎)" : "Open it (⏎)"}>
            <Icon name={reads || isArtifact(a) ? "arrow-up-right" : "external"} /> {reads ? "Open its page" : "Open it"} <Keys k="⏎" />
          </Button>
        }
        title={handoff ? (desc ?? title) : title}
        meta={
          <>
            {(home.crew || a.by) && (
              <span>
                <CrewBadge id={home.crew?.id ?? a.by!} live={home.crew ? !home.crew.ended : false} /> {handoff ? "left it" : "made it"}
              </span>
            )}
            {home.capsule && (
              <Button variant="link" onClick={() => select(home.capsule!.key)} title={home.capsule.id}>
                <Icon name="box" size={12} /> {label(home.capsule.c.repo)}
                {home.capsule.c.name !== d.slug && <span className="mono"> {home.capsule.c.name}</span>}
              </Button>
            )}
            <MissionOutputState state={a.state} />
            {a.threads && <MissionOutputThreads t={a.threads} />}
            {!handoff && desc && <span>{desc}</span>}
          </>
        }
      />
      <MissionWorkBody fills>
        {error && !doc && <ErrorCard title="Couldn't read it" error={error} />}
        {reads && !doc && !error && <Skeleton rows={6} />}
        {doc &&
          (doc.media ? (
            <MissionWorkEmpty icon="file" title="It's an image or a video">
              Its page shows it as it is.
            </MissionWorkEmpty>
          ) : doc.markdown ? (
            <MissionWorkProse>
              <Markdown text={text} />
            </MissionWorkProse>
          ) : (
            <MissionWorkProse code>{doc.text}</MissionWorkProse>
          ))}
        {!reads && (
          <MissionWorkEmpty icon={isArtifact(a) ? "artifact" : "external"} title={isArtifact(a) ? "An artifact: it runs on its own page" : a.url ? "It's kept elsewhere" : "There's nothing of it to read here"}>
            {desc}
          </MissionWorkEmpty>
        )}
      </MissionWorkBody>
    </>
  );
}
