import type { ReactNode } from "react";
import { Link } from "react-router";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import { PageHead } from "../ui/PageHead";
import css from "./SpaceHead.module.css";

type SpaceHeadProps = {
  label: string;
  color: string; // its repo's
  folder?: string; // the space's own, when it's a folder of its repo
  count: number; // its documents, on its default branch
  fresh?: ReactNode; // how it stands with its remote, a DocumentsFresh
  actions: ReactNode; // New
};

// SpaceHead is a space's page's head: the way back to Documents (esc),
// its name, how many documents it has and how it stands with its remote.
export function SpaceHead({ label, color, folder, count, fresh, actions }: SpaceHeadProps) {
  return (
    <PageHead
      className={css.head}
      lead={
        <>
          <Link to="/documents" className={css.back} {...keyProps("esc", "Back to Documents")}>
            <Icon name="arrow-left" size={13} /> Documents
          </Link>
          <span className={css.slash} aria-hidden>
            /
          </span>
        </>
      }
      title={
        <>
          <span className="repo-dot" style={{ background: color }} /> {label}
          {folder && <span className="mono dim"> {folder}</span>}
        </>
      }
      actions={actions}
    >
      <span className={css.count} title="As on its default branch">
        {count} {count === 1 ? "document" : "documents"}
      </span>
      {fresh}
    </PageHead>
  );
}
