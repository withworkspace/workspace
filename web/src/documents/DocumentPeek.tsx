import type { ReactNode } from "react";
import { Link } from "react-router";
import { Chip } from "../ui/Chip";
import { Copyable } from "../ui/Copyable";
import { Icon } from "../ui/Icon";
import css from "./DocumentPeek.module.css";

export type DocumentPeekProps = {
  // Where it is: the repo (as the workspace calls it), the capsule its
  // text was read from if it's not the repo's default branch, its path.
  repo: string;
  capsule?: string;
  path: string;
  // Roughly how long it is.
  words: number;
  // The mission that records it, if one does: its threads are there.
  mission?: { title: string; to: string };
  // Its text, as it's read: without its title, which the pane's head has.
  children: ReactNode;
};

// DocumentPeek is a document as it's read beside another page: where it
// is, how long it is, and what it says. It's for reading: asking about it
// and changing it are its own page's, at the pane's head.
export function DocumentPeek({ repo, capsule, path, words, mission, children }: DocumentPeekProps) {
  return (
    <div className={css.peek}>
      <div className={css.where}>
        <Chip tone="quiet">{repo}</Chip>
        {capsule && (
          <Chip tone="progress" title={`As it is in the capsule ${capsule}, not on the repo's default branch`}>
            in {capsule}
          </Chip>
        )}
        <Copyable text={path} className={css.path} title="Copy its path" icon="over">
          {path}
        </Copyable>
        <span className={css.words}>
          {words.toLocaleString()} {words === 1 ? "word" : "words"}
        </span>
      </div>
      {mission && (
        <Link to={mission.to} className={css.mission}>
          <Icon name="flag" size={12} /> A document of {mission.title}
        </Link>
      )}
      <article className={css.text}>{children}</article>
    </div>
  );
}
