import type { ArtifactInfo, TalkProposal } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Suggestion } from "../review/Suggestion";
import { Excerpt } from "../ui/Excerpt";
import { ArtifactProposalBody } from "./ArtifactProposal";
import { BriefDiff } from "./BriefDiff";
import { TalkProposalMeta } from "./TalkProposalMeta";
import { TalkProposalPlan } from "./TalkProposalPlan";
import { TalkProposalWhat } from "./TalkProposalWhat";
import { TalkProposalWhere } from "./TalkProposalWhere";
import { TalkProposalWhole } from "./TalkProposalWhole";
import { Text } from "../ui/Text";
import { Prose } from "../ui/Prose";

// ProposalShown is what a card of ws's own proposes, as it was proposed
// (not being edited), by its kind. briefing: a brief that starts its
// mission; prHref and lines, where a review comment goes; frame, where an
// artifact proposed runs; artifact, the one a version's of.
export function ProposalShown({ p, briefing, prHref, lines, frame, artifact }: { p: TalkProposal; briefing: boolean; prHref: string; lines: string; frame: string; artifact?: ArtifactInfo }) {
  switch (p.kind) {
    case "decision":
      return (
        <>
          <TalkProposalWhat>{p.what}</TalkProposalWhat>
          {p.why && <TalkProposalWhat why>{p.why}</TalkProposalWhat>}
        </>
      );
    case "mission":
      return (
        <>
          {p.brief ? <TalkProposalWhole text={p.brief} /> : <Text as="div" dim>Idea to brief later, starting from this conversation.</Text>}
          <TalkProposalMeta items={[p.about?.length ? `About ${p.about.join(", ")}` : undefined, p.capsules?.length ? `Lifts ${p.capsules.join(", ")}` : undefined]} />
        </>
      );
    case "brief":
      if (briefing)
        return (
          <>
            <TalkProposalWhole text={p.brief ?? ""} />
            {p.plan ? <TalkProposalPlan text={p.plan} /> : <Text as="div" dim>No plan: crew work from the brief.</Text>}
            <TalkProposalMeta items={[p.capsules?.length ? `Lifts ${p.capsules.join(", ")}` : undefined, p.docks?.length ? `Builds on ${p.docks.join(", ")}` : undefined, p.slug && `Renamed ${p.slug}`]} />
          </>
        );
      return <BriefDiff was={p.was ?? ""} now={p.brief ?? ""} />;
    case "plan":
      return (
        <>
          {p.why && <TalkProposalWhat why>{p.why}</TalkProposalWhat>}
          {p.was ? (
            <BriefDiff was={p.was} now={p.plan ?? ""} />
          ) : (
            <Prose>
              <Markdown text={p.plan ?? ""} />
            </Prose>
          )}
          {p.step ? <TalkProposalMeta items={[`For step ${p.step}: accepting passes that step`]} /> : null}
        </>
      );
    case "dispatch":
      return (
        <>
          <TalkProposalWhat said>{p.text}</TalkProposalWhat>
          <TalkProposalMeta items={[p.capsule ? `In ${p.capsule}` : "In a capsule with no crew", p.reads?.length ? `Reading ${p.reads.join(", ")}` : undefined]} />
        </>
      );
    case "replan":
      return (
        <>
          <TalkProposalWhat said>{p.text}</TalkProposalWhat>
          {p.state === "open" && <Text as="div" dim>Confirming sends this to the planner as you.</Text>}
        </>
      );
    case "comment":
      return (
        <>
          <TalkProposalWhere kind={p.noteKind} pr={p.pr} href={prHref} path={p.path} lines={lines} removed={p.side === "old"} />
          {!!p.excerpt?.length && <Excerpt lines={p.excerpt} from={p.line ?? 1} />}
          {p.text && <TalkProposalWhat said>{p.text}</TalkProposalWhat>}
          {p.suggestion != null && <Suggestion from={p.excerpt} to={p.suggestion} />}
        </>
      );
    case "document":
      // An edit to one: what it changes, from the document as the agent
      // read it (saving merges in what's changed there since).
      if (p.to)
        return (
          <>
            <BriefDiff was={p.was ?? ""} now={p.text ?? ""} />
            <TalkProposalMeta items={[`Changes ${p.to}`, p.subject && `Committed as “${p.subject}”`, p.guide && `Written as ${p.guide} says`]} />
          </>
        );
      // Shown whole: adding it commits it, where its kind says.
      return (
        <>
          <TalkProposalWhole text={p.text ?? ""} />
          <TalkProposalMeta items={[p.path && `Adds ${p.path}`, p.subject && `Committed as “${p.subject}”`, p.guide && `Written as ${p.guide} says`]} />
        </>
      );
    case "artifact":
      return <ArtifactProposalBody p={p} src={frame} artifact={artifact} />;
  }
  return null;
}
