import type { PluginProposalItem, TalkProposal } from "../api";
import { Inline } from "../ui/Inline";
import { TalkProposalSettled } from "./TalkProposalSettled";
import { Text } from "../ui/Text";

// PluginOutcome is what came of a plugin's proposal, in its foot: that it
// was superseded, withdrawn or declined; or what was done (made: its
// items the plugin made), and why the rest weren't.
export function PluginOutcome({ p, made }: { p: TalkProposal; made: PluginProposalItem[] }) {
  switch (p.state) {
    case "superseded":
      return (
        <TalkProposalSettled icon="arrow-down">
          <span>
            Superseded by <a href={`#proposal-${p.result}`}>proposal {p.result}</a>, proposed after it: nothing to confirm here
          </span>
        </TalkProposalSettled>
      );
    case "withdrawn":
      return (
        <TalkProposalSettled icon="x">
          {" "}Withdrawn
          {p.because ? `: ${p.because}` : ""}
        </TalkProposalSettled>
      );
    case "declined":
      return <TalkProposalSettled icon="x"> Declined</TalkProposalSettled>;
  }
  const names = made.map((it) => it.made).filter(Boolean) as string[];
  return (
    <TalkProposalSettled done icon="check-circle" size={14}>
      {" "}
      {names.length ? <Inline text={`Done: ${names.join(", ")}`} /> : "Done"}
      {p.because && <Text dim> (the rest withdrawn: {p.because})</Text>}
    </TalkProposalSettled>
  );
}
