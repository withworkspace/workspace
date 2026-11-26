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
          Superseded by <a href={`#proposal-${p.result}`}>proposal {p.result}</a>
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
  const open = (to: string, words = "Open") => (
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
      return done(briefing ? (p.plan ? "Ready for crew" : "Briefed: ready for crew") : "Brief updated");
    case "plan":
      return done("Plan updated");
    case "dispatch":
      return done(p.result === "queued" ? "Queued: launches from the mission page" : `Started ${p.result}`);
    case "replan":
      return (
        <>
          {done(`Asked the planner in ${p.result}`)}
          {slug && p.result && open(talkHref(slug, p.result))}
        </>
      );
    case "comment":
      return (
        <>
          {done("Added to draft review: posts when you submit")}
          {!inReview && open(prHref, "Open review")}
        </>
      );
    case "document": {
      // Landed (repo:path), or left in a capsule (repo/capsule:path) to
      // open a pull request from. An edit to one in a capsule was saved
      // there, where it was.
      const capsule = p.result?.split(":")[0].includes("/") ? p.result.split(":")[0] : "";
      const leftIn = capsule && p.result !== p.to ? `in ${capsule}, ready for a pull request` : "";
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
  if (!p.result) return done("Decided: saved with this conversation");
  const [on, ref] = p.result.split(":");
  return done(
    <>Recorded as {ref} on <Link to={`/missions/${on}`} className={textClass("link")}>{on}</Link></>,
  );
}
