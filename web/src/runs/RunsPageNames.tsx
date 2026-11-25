import type { ReactNode } from "react";
import { Link } from "react-router";
import { Summary, SummaryRow } from "../ui/Summary";
import css from "./RunsPageNames.module.css";
import { Text } from "../ui/Text";

// RunsPageNames is where the workspace's names stand, with names on: where
// this page and its runs are reached (names), why it's another name than
// the one it wants (note), the router serving them (router), and the way
// to the setting.
export function RunsPageNames({ names, note, router }: { names: ReactNode; note?: ReactNode; router: ReactNode }) {
  return (
    <Summary className={css.names}>
      <SummaryRow label="Names">
        {names}
        {note && <Text as="div" dim className={css.note}>{note}</Text>}
      </SummaryRow>
      <SummaryRow label="Router">{router}</SummaryRow>
      <SummaryRow label="Settings">
        <Link to="/settings/runs">Local names, in Settings</Link>
      </SummaryRow>
    </Summary>
  );
}
