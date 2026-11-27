import { useEffect, useState } from "react";
import type { PassMark, PR, ReviewRow, Stack } from "../api";
import { useThingMenu } from "../shell/ThingMenu";
import { Chip } from "../ui/Chip";
import { Icon } from "../ui/Icon";
import { RowPasses } from "./RowPasses";
import { useReviewsMemory } from "./memory";
import { prHref, prStatus, rowTarget } from "./Reviews";
import { ReviewsLayers } from "./ReviewsLayers";
import { ReviewsRow } from "./ReviewsRow";
import { merge } from "./merge";
import { PullRequestMerge } from "./PullRequestMerge";
import { offerMerge } from "./mergeOffers";

// PullRow draws a row of the Reviews page, a pull request or a stack of
// them, as a ReviewsRow: what it is, where it opens, and a stack's layers.
export function PullRow({
  row,
  selected,
  folded,
  onHover,
  onOpen,
  onFold,
  passes,
}: {
  row: ReviewRow;
  selected: boolean;
  folded: boolean;
  onHover: () => void;
  onOpen: () => void;
  onFold: () => void;
  passes?: Record<string, PassMark[]>; // how passes stand, by pull request
}) {
  const menu = useThingMenu();
  const target = rowTarget(row);
  const pr = row.pr;
  const st = row.stack;
  // (A field GitHub left out is absent, so each is read from whichever
  // the row is, not from the first that has it.)
  const of = <T,>(a: (p: PR) => T, b: (s: Stack) => T): T => (pr ? a(pr) : b(st!));
  const author = of((p) => p.author, (s) => s.author);
  const mine = of((p) => p.mine, (s) => s.mine);
  const mission = of((p) => p.mission, (s) => s.layers.find((l) => l.mission)?.mission);
  const onTopOf = of((p) => p.onTopOf, (s) => s.onTopOf);
  // Yours ready to merge merges from its row, a stack whole from its top;
  // one left to merge by itself says so. Leaving one to is its page's.
  const [merging, setMerging] = useState(false);
  const top = st?.merges ? st.layers[st.layers.length - 1] : undefined;
  const asked = top ?? (pr?.merge === "now" || pr?.autoMerge ? pr : undefined);
  const mergeIt = (when: "now" | "off") => {
    if (!asked) return;
    setMerging(true);
    void merge(asked.repo, asked.number, when, asked.head).finally(() => setMerging(false));
  };
  // What its menu (Find's acts) offers to merge, by each pull request it
  // is: a stack's layers each merge it whole.
  const offerKey = [pr?.merge, pr?.autoMerge, pr?.head, top?.number, top?.head].join("|");
  useEffect(() => {
    const refs: string[] = [];
    if (top && st) {
      for (const l of st.layers) {
        refs.push(`${l.repo}#${l.number}`);
        offerMerge(refs[refs.length - 1], { offer: "now", head: top.head, stack: st.number, layers: st.layers.length, repo: top.repo, number: top.number });
      }
    } else if (pr && (pr.merge || pr.autoMerge)) {
      refs.push(`${pr.repo}#${pr.number}`);
      offerMerge(refs[0], { offer: pr.autoMerge ? "on" : pr.merge!, head: pr.head, repo: pr.repo, number: pr.number });
    }
    return () => refs.forEach((r) => offerMerge(r, undefined));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [offerKey]);
  const act = asked && (
    <PullRequestMerge
      offer={!top && pr?.autoMerge ? "on" : "now"}
      stack={!!top}
      busy={merging}
      onMerge={() => mergeIt("now")}
      onStop={() => mergeIt("off")}
    />
  );
  return (
    <ReviewsRow
      id={`pr-row-${row.key}`}
      selected={selected}
      status={prStatus(pr ?? st!)}
      title={of((p) => p.title, (s) => s.title)}
      chips={
        <>
          {st && (
            <Chip tone="quiet" title={st.workedOut ? "Detected stack: each pull request builds on the one before" : `Stack ${st.number} on GitHub`}>
              <Icon name="layers" size={11} /> stack of {st.layers.length}
            </Chip>
          )}
          {pr?.draft && <Chip tone="quiet">Draft</Chip>}
          {pr?.autoMerge && (
            <Chip tone="quiet" title="It merges by itself once its checks and reviews let it">
              <Icon name="merge" size={11} /> merges when ready
            </Chip>
          )}
          {pr?.asked === "team" && (
            <Chip tone="quiet" title="Requested from your team">
              Your team
            </Chip>
          )}
          {!!onTopOf && (
            <Chip tone="quiet" title="Based on that pull request's branch">
              on top of #{onTopOf}
            </Chip>
          )}
          {mission && (
            <Chip tone="quiet" title={`From mission ${mission.title}`}>
              <Icon name="flag" size={11} /> {mission.title}
            </Chip>
          )}
          {pr?.capsule && (
            <Chip tone="quiet" title={`Checked out in ${pr.repo}/${pr.capsule}`}>
              <Icon name="box" size={11} /> {pr.capsule}
            </Chip>
          )}
          {pr && <RowPasses marks={passes?.[`${pr.repo}#${pr.number}`]} />}
        </>
      }
      sub={
        <>
          {mine ? "You" : author} · {of((p) => p.repo, (s) => s.repo)}
          {pr && <> · #{pr.number}</>} · {of((p) => p.words, (s) => s.words)}
          {st?.merged ? ` · ${st.merged} merged below` : ""}
        </>
      }
      additions={of((p) => p.additions, (s) => s.additions)}
      deletions={of((p) => p.deletions, (s) => s.deletions)}
      files={pr?.files}
      checks={pr?.checks}
      updated={of((p) => p.updated, (s) => s.layers.reduce((a, l) => (l.updated && l.updated > a ? l.updated : a), ""))}
      to={target.to}
      href={target.href}
      onHover={onHover}
      onOpen={target.to ? () => useReviewsMemory.setState({ selected: row.key }) : onOpen}
      onContextMenu={target.to ? menu(target.to) : undefined}
      fold={st ? { folded, onFold } : undefined}
      act={act || undefined}
      layers={
        st && !folded ? (
          <ReviewsLayers
            layers={st.layers.map((l) => ({
              number: l.number,
              status: prStatus(l),
              title: l.title,
              words: l.words,
              passes: <RowPasses marks={passes?.[`${l.repo}#${l.number}`]} />,
              additions: l.additions,
              deletions: l.deletions,
              checks: l.checks,
              ...(l.mine && !l.mission ? { href: l.url } : { to: l.mine && l.mission ? `/missions/${l.mission.slug}` : prHref(l.repo, l.number) }),
            }))}
          />
        ) : undefined
      }
    />
  );
}
