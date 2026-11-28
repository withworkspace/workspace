import { useEffect } from "react";
import { getPR, type PRDetail } from "../api";
import { DockBody } from "../chrome/DockBody";
import { PaneHead } from "../chrome/PaneHead";
import { useTabNames } from "../chrome/tabNames";
import { useLoaded } from "../load";
import { Markdown } from "../markdown/LazyMarkdown";
import { PullRequestPeek } from "../reviews/PullRequestPeek";
import { prHref, prStatus } from "../reviews/prs";
import { ErrorCard } from "../ui/ErrorCard";
import { Loading } from "../ui/Loading";
import { humanError } from "../ui/strings";
import { Text } from "../ui/Text";

type PullRequestPaneProps = {
  // Its tab's address, which is its review's.
  to: string;
  repo: string;
  number: number;
};

// PullRequestPane is a pull request, peeked at beside the page: what
// GitHub last said of it, drawn as a PullRequestPeek. Its diff and your
// review are its own page's.
export function PullRequestPane({ to, repo, number }: PullRequestPaneProps) {
  const { value: d, error, reload } = useLoaded<PRDetail>(() => getPR(repo, number), [repo, number]);
  const pr = d?.pr;
  const name = `${repo}#${number}`;
  useEffect(() => {
    if (pr) useTabNames.getState().say(to, `#${pr.number} ${pr.title}`);
  }, [to, pr]);
  return (
    <DockBody label={pr?.title ?? name} pad>
      <PaneHead title={pr?.title ?? name} to={to} sub={<Text mono>{name}</Text>} />
      {error && !d && <ErrorCard title="Could not load pull request" error={humanError(error)} retry={() => void reload()} />}
      {!d && !error && <Loading what="Loading pull request…" />}
      {d && pr && (
        <PullRequestPeek
          status={prStatus(pr)}
          author={pr.author}
          mine={pr.mine}
          draft={pr.draft}
          branch={pr.branch}
          base={pr.base}
          additions={pr.additions}
          deletions={pr.deletions}
          files={pr.files}
          checks={pr.checks}
          updated={pr.updated}
          mission={pr.mission && { title: pr.mission.title, to: `/missions/${encodeURIComponent(pr.mission.slug)}` }}
          layers={d.stack?.layers.map((l) => ({ number: l.number, title: l.title, status: prStatus(l), to: prHref(l.repo, l.number), here: l.number === pr.number }))}
        >
          {d.body?.trim() ? <Markdown text={d.body} /> : undefined}
        </PullRequestPeek>
      )}
    </DockBody>
  );
}
