import { Link, useParams } from "react-router";
import { capsuleAPI, getDiff, submitReview } from "../api";
import { Callout } from "../ui/Callout";
import { GuideMark } from "../reviews/GuideMark";
import { GuideSection } from "../reviews/GuideSection";
import { MissionContext } from "../reviews/MissionContext";
import { PassesSection } from "../reviews/PassesSection";
import { PassMarks } from "../reviews/PassMarks";
import { useReviewPasses } from "../reviews/usePasses";
import { useWorkspace } from "../workspace/store";
import { LandBar } from "./LandBar";
import { ReviewSubmit } from "./ReviewSubmit";
import { ReviewView } from "./ReviewView";
import type { ReviewSource } from "./source";
import { verdictWords } from "./words";
import { Text } from "../ui/Text";
import { hoverCardLinkClass } from "../ui/HoverCard";
import { Icon } from "../ui/Icon";
import { headlineStatus } from "../ui/statusOf";
import { ReviewFact } from "./ReviewFact";
import { ReviewFacts } from "./ReviewFacts";
import { ReviewRepo } from "./ReviewRepo";
import { ReviewStand } from "./ReviewStand";

// How a file's lines are coloured is asked for by this page's name.
export { highlight } from "./highlightDiff";

// Review is a mission capsule's review.
export function Review() {
  const { slug = "", repo = "", name = "" } = useParams();
  const api = capsuleAPI(slug, repo, name);
  // Its passes and its guide: as a pull request's review has.
  const rp = useReviewPasses(api);
  const mission = useWorkspace((w) => w.missions.find((m) => m.slug === slug));
  const current = mission?.capsules.find((c) => c.repo === repo && c.name === name);
  const defaultBranch = useWorkspace((w) => w.defaultBranch) || "main";
  if (!mission || !current) {
    return (
      <div className="page">
        <h1>No such capsule</h1>
        <Text as="p" dim>
          {slug} has no capsule {repo}/{name}.
        </Text>
      </div>
    );
  }
  const source: ReviewSource = {
    api,
    title: `Review ${repo}/${name}`,
    // Where its mission stands, its name, and the repo it's in. Said in
    // the card: its branch, and the mission it's for.
    stand: (
      <ReviewStand s={headlineStatus(mission.headline)}>
        <ReviewFacts>
          {current.branch && (
            <ReviewFact term="Branch">
              <Text mono>{current.branch}</Text> <Icon name="arrow-right" size={11} /> <Text mono>{defaultBranch}</Text>
            </ReviewFact>
          )}
          <ReviewFact term="Mission">
            <Link to={`/missions/${slug}`} className={hoverCardLinkClass()}>
              {mission.title}
            </Link>
          </ReviewFact>
        </ReviewFacts>
      </ReviewStand>
    ),
    name: <Text mono>{name}</Text>,
    about: <ReviewRepo repo={repo} />,
    back: { to: `/missions/${slug}`, label: mission.title },
    headNow: current.head,
    updated: `${mission.updated}:${rp.took}`,
    // A capsule has no description of its own: its overview is its
    // mission, whose brief says what it's for. Then its guide and passes.
    // (Nothing's said on GitHub about a capsule.)
    views: [
      { key: "overview", body: <MissionContext mission={mission} capsule={current} /> },
      { key: "guide", mark: <GuideMark guide={rp.guide} />, body: <GuideSection api={api} guide={rp.guide} guided={rp.guided} onGuide={rp.setGuide} onGuided={rp.setGuidedKept} /> },
      {
        key: "passes",
        mark: <PassMarks passes={rp.passes} />,
        body: (go) => (
          <PassesSection
            api={api}
            passes={rp.passes}
            hidden={rp.hidden}
            canAsk={false}
            toFinding={go.toFinding}
            toComment={go.toComment}
            onResolve={() => Promise.resolve()}
            onPass={rp.putPass}
            onTook={rp.bumpTook}
            onHide={rp.hide}
            onAsk={() => undefined}
          />
        ),
      },
    ],
    order: rp.order,
    onOrder: rp.guide?.guide ? rp.setGuidedKept : undefined,
    guideNotes: rp.notes,
    findings: rp.findings,
    onFinding: rp.onFinding,
    // Since your last review, when there's been work since.
    sinceOf: (latest) => (latest && current.head && latest.head !== current.head ? latest.head : undefined),
    getDiff: (since) => getDiff(api, since),
    movedWords: "Capsule has new commits",
    empty: "No committed changes yet.",
    browse: `/missions/${slug}/files/${repo}/${name}?only=changed`,
    // A repo with no pull requests: this is the only review there is, and
    // it lands from here.
    notices: <LandBar repo={repo} name={name} updated={`${mission.updated}:${current.head}`} back={`/missions/${slug}?tab=work`} />,
    submit: ({ draft, diff, onClose, onDiscard, done }) => (
      <ReviewSubmit
        draft={draft}
        onClose={onClose}
        onDiscard={onDiscard}
        onSubmit={async (verdict, summary) => done(await submitReview(api, { base: diff.base, head: diff.head }, verdict, summary))}
      />
    ),
    submitted: (r) => (
      <Callout tone="done" title={`Submitted ${r.id}: ${verdictWords[r.verdict!]}`}>
        The orchestrator is deciding what's next.
      </Callout>
    ),
  };
  return <ReviewView key={api} source={source} />;
}
