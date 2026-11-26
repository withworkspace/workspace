import type { ReactNode } from "react";
import { Copyable } from "../../ui/Copyable";
import { Time } from "../../ui/Time";
import { MissionOutputState } from "../MissionOutputState";
import css from "./MissionArtifactMeta.module.css";

type MissionArtifactMetaProps = {
  // What kind of document it is, in a word, and (hint) what that kind is.
  kind: string;
  hint?: string;
  // Where it stands: draft, in review, landed…
  state: string;
  by?: string;
  added?: string;
  // Its ref, to copy.
  docRef: string;
  // What's done to it, at the line's end: edit it, open it where it lives.
  actions?: ReactNode;
};

// MissionArtifactMeta is the line over one of a mission's documents: its
// kind, where it stands, who made it and when, its ref to copy, and what's
// done to it.
export function MissionArtifactMeta({ kind, hint, state, by, added, docRef, actions }: MissionArtifactMetaProps) {
  return (
    <div className={css.meta}>
      <span className={css.kind} title={hint}>
        {kind}
      </span>
      <MissionOutputState state={state} />
      {by && <span>by {by}</span>}
      {added && <Time iso={added} as="relative" />}
      <Copyable text={docRef} className={css.ref} title="Copy ref">
        {docRef.replace(":", " · ")}
      </Copyable>
      <span className={css.spacer} />
      {actions}
    </div>
  );
}
