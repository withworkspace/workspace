import type { ReactNode } from "react";
import { Link } from "react-router";
import { docAddress, type TalkProposal } from "../api";
import { RefTag } from "../refs/RefTag";
import { refKey } from "../refs/refs";
import { buttonClass } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { addedVersion } from "./proposals";
import { TalkProposalSettled } from "./TalkProposalSettled";
import { talkHref } from "./words";
import { textClass } from "../ui/Text";

// ProposalOutcome is what came of a card of ws's own, in its foot: that it
// was declined, superseded or withdrawn; or what accepting it did (as a
// TalkProposalSettled, done), with a way to what it made. briefing: a
// brief that started its mission; slug, the mission the conversation's
// about; prHref, a review comment's pull request, which inReview it's
// already on.
export function ProposalOutcome({ p, briefing, slug, prHref, inReview }: { p: TalkProposal; briefing: boolean; slug?: string; prHref: string; inReview?: boolean }) {
  if (p.state === "declined")
    return <TalkProposalSettled icon="x"> Declined</TalkProposalSettled>;
  if (p.state === "superseded")
    return (
      <TalkProposalSettled icon="arrow-down">
        <span>
          Superseded by <a href={`#proposal-${p.result}`}>proposal {p.result}</a>, proposed after it: nothing to confirm here
        </span>
      </TalkProposalSettled>
    );
  if (p.state === "withdrawn")
    return <TalkProposalSettled icon="x"> Withdrawn{p.because ? `: ${p.because}` : ""}</TalkProposalSettled>;
  const done = (children: ReactNode) => (
    <TalkProposalSettled done icon="check-circle" size={14}>
      {" "}
      {children}
    </TalkProposalSettled>
  );
  const open = (to: string, words = "Open it") => (
    <Link to={to} className={buttonClass()}>
      {words} <Icon name="arrow-right" size={13} />
    </Link>
  );
  switch (p.kind) {
    case "mission":
      return p.result ? (
        <>
          {done("Mission created")}
          {open(`/missions/${p.result}`)}
        </>
      ) : (
        done("Saved as an idea")
      );
    case "brief":
      return done(briefing ? (p.plan ? "Briefed and planned: it's ready for crew" : "Briefed: it's ready for crew") : "The brief is updated");
    case "plan":
      return done("The plan is updated");
    case "dispatch":
      return done(p.result === "queued" ? "Queued: the mission's page launches it" : `Started ${p.result}`);
    case "replan":
      return (
        <>
          {done(`Asked the planner, in ${p.result}: it revises the plan, or says why not`)}
          {slug && p.result && open(talkHref(slug, p.result))}
        </>
      );
    case "comment":
      return (
        <>
          {done("In your draft review: it's posted when you submit")}
          {!inReview && open(prHref, "Open the review")}
        </>
      );
    case "document": {
      // Landed (repo:path), or left in a capsule (repo/capsule:path) to
      // open a pull request from. An edit to one in a capsule was saved
      // there, where it was.
      const capsule = p.result?.split(":")[0].includes("/") ? p.result.split(":")[0] : "";
      const leftIn = capsule && p.result !== p.to ? `in a capsule, ${capsule}, to open a pull request from` : "";
      return (
        <>
          {done(p.to ? (leftIn ? `Saved ${leftIn}` : "Saved") : leftIn ? `Added ${leftIn}` : "Added")}
          {p.result && open(docAddress(p.result))}
        </>
      );
    }
    case "artifact": {
      // Added, as a version: a3.v1, drawn as its card.
      const v = addedVersion(p.result);
      return v ? (
        <>
          {done(
            <span>
              Added as{" "}
              <RefTag refKey={refKey({ kind: "artifact", id: v.id, version: v.n })} inline>
                {p.result}
              </RefTag>
            </span>,
          )}
          {open(`/artifacts/${v.id}/v${v.n}`)}
        </>
      ) : (
        done("Added")
      );
    }
  }
  if (!p.result) return done("Decided: kept with this conversation until it becomes a mission");
  const [on, ref] = p.result.split(":");
  return done(
    <>Recorded as {ref} on <Link to={`/missions/${on}`} className={textClass("link")}>{on}</Link></>,
  );
}
