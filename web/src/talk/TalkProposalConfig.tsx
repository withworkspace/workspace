import { changes } from "../annotate/diff";
import { Change } from "../ui/Change";
import { Text } from "../ui/Text";
import css from "./TalkProposalConfig.module.css";

// TalkProposalConfig is what a change to ws's config does to its file
// (ws.toml, a repo's ws.repo.toml): each changed passage as code, the
// lines it takes out over those it puts in; a file that isn't there yet,
// whole, as lines put in.
export function TalkProposalConfig({ was, now }: { was: string; now: string }) {
  const lines = now.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  if (!was.trim()) return <Change className={css.config} signs now={lines} />;
  const hunks = changes(was, now);
  if (!hunks.length) return <Text as="div" dim>No change.</Text>;
  return <Change className={css.config} signs hunks={hunks.map((h) => ({ was: h.old, now: lines.slice(h.newStart - 1, h.newEnd) }))} />;
}
