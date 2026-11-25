import type { ReactNode } from "react";
import { Link } from "react-router";
import { Count } from "../../ui/Count";
import { cx } from "../../ui/cx";
import { GroupHead, groupHeadClass } from "../../ui/GroupHead";
import css from "./FilesRepoGroup.module.css";

type FilesRepoGroupProps = {
  // shown: under its name (none for the ungrouped, first); hidden: folded
  // away, said to be hidden; documents: the repos that are spaces of
  // documents, folded at the foot, with the way to Documents.
  kind: "shown" | "hidden" | "documents";
  label?: string;
  count: number;
  // A folded group is open (what's open is in it).
  open?: boolean;
  children: ReactNode;
};

// FilesRepoGroup is a group of the workspace's repos (FilesRepoRows), as
// ws.toml groups them.
export function FilesRepoGroup({ kind, label, count, open, children }: FilesRepoGroupProps) {
  if (kind === "shown")
    return (
      <section className={css.group}>
        {label && (
          <GroupHead className={css.head} count={count}>
            {label}
          </GroupHead>
        )}
        {children}
      </section>
    );
  if (kind === "hidden")
    return (
      <details className={css.group} open={open}>
        <summary className={groupHeadClass(css.head)}>
          {label} <span className="dim">· hidden</span> <Count>{count}</Count>
        </summary>
        {children}
      </details>
    );
  return (
    <details className={cx(css.group, css.documents)} open={open}>
      <summary className={groupHeadClass(css.head)} title="Repos of documents: read in Documents, by their titles. Their files are here too.">
        In Documents <Count>{count}</Count>
        <Link to="/documents" className={css.link} title="Open Documents" onClick={(e) => e.stopPropagation()}>
          Open
        </Link>
      </summary>
      {children}
    </details>
  );
}
