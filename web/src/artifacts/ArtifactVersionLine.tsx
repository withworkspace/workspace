import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";
import { Time } from "../ui/Time";
import css from "./ArtifactVersionLine.module.css";

type ArtifactVersionLineProps = {
  // Which version it is, of how many.
  n: number;
  of: number;
  // The version it's based on, where that isn't the one before.
  basedOn?: number;
  // Where it came from (a conversation, a mission), drawn, and the
  // proposal it was there.
  from?: ReactNode;
  proposal?: number;
  // Who added it, as it's said: "by you, by hand", "by c1".
  by: string;
  at: string;
  // How much it is: "3 files, 240 KB".
  size?: string;
  // Where it's kept: the repo, when it's the version that lives there;
  // this machine otherwise. Its title says so in full.
  kept: { repo?: string; title: string };
};

// ArtifactVersionLine is a version in a line: v2 of 3, based on v1, from
// where, by whom, when, how much, and where it's kept, a dot between.
export function ArtifactVersionLine({ n, of, basedOn, from, proposal, by, at, size, kept }: ArtifactVersionLineProps) {
  return (
    <div className={css.line}>
      <span>
        v{n} of {of}
      </span>
      {basedOn ? <span>based on v{basedOn}</span> : null}
      {from && (
        <span>
          from {from}
          {proposal ? `, proposal ${proposal}` : ""}
        </span>
      )}
      <span>{by}</span>
      <Time iso={at} as="relative" />
      {size ? <span>{size}</span> : null}
      {kept.repo ? (
        <span title={kept.title}>
          <Icon name="branch" size={11} /> in {kept.repo}
        </span>
      ) : (
        <span title={kept.title}>
          <Icon name="monitor" size={11} /> This machine
        </span>
      )}
    </div>
  );
}
