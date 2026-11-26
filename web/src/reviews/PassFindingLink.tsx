import type { ReactNode } from "react";
import { Button } from "../ui/Button";
import css from "./PassFindingLink.module.css";

// PassFindingLink goes to a finding in the diff, or, where its lines
// aren't in it, shows them under its head.
export function PassFindingLink({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <Button variant="link" className={css.link} onClick={onClick} title="Show lines">
      {children}
    </Button>
  );
}
