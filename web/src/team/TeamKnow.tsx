import type { Know } from "../api";
import { Inline } from "../ui/Inline";
import { TeamKnowFile, TeamKnowLine } from "./TeamKnowLine";
import { fileHref, knowInfo } from "./words";

// TeamKnow is one thing worth knowing of a change, under its row: its
// kind, what it is, and the file it rests on, which opens in Files.
export function TeamKnow({ know: k, repo }: { know: Know; repo: string }) {
  const info = knowInfo[k.kind] ?? knowInfo.behaviour;
  return (
    <TeamKnowLine kind={{ ...info, tone: "attention" }}>
      <Inline text={k.text} />{" "}
      <TeamKnowFile to={fileHref(k.in || repo, k.file)} title={`Open ${k.file} in Files`}>
        {k.in ? `${k.in}: ` : ""}
        {k.file.split("/").pop()}
      </TeamKnowFile>
    </TeamKnowLine>
  );
}
