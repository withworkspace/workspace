import type { ReactNode } from "react";
import css from "./PassCard.module.css";
import { Text } from "../ui/Text";

// PassCard is one of a review's passes: how it stands, its title, what
// it's for (about) and what's done with it (tools) in its head, then what
// it's said and found.
export function PassCard({ standing, title, about, tools, children }: { standing: ReactNode; title: string; about?: string; tools: ReactNode; children?: ReactNode }) {
  return (
    <article className={css.pass}>
      <header>
        {standing}
        <strong>{title}</strong>
        <Text dim className={css.about}>{about}</Text>
        <span className={css.spacer} />
        {tools}
      </header>
      {children}
    </article>
  );
}
