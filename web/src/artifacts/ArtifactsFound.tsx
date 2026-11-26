import type { ReactNode } from "react";
import css from "./ArtifactsFound.module.css";
import { Text } from "../ui/Text";

// ArtifactsFound is the artifacts ws found in the workspace's repos, under
// the Artifacts page's own: what they are, over the list or the tree of
// them (its children).
export function ArtifactsFound({ children }: { children: ReactNode }) {
  return (
    <section className={css.found} aria-labelledby="artifacts-found-title">
      <h2 id="artifacts-found-title" className={css.title}>
        In the workspace's repos
      </h2>
      <Text as="p" dim className={css.about}>Promoted by someone else, or by you on another machine: each runs from its repo's files.</Text>
      {children}
    </section>
  );
}
