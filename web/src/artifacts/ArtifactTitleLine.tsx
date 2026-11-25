import type { ReactNode } from "react";
import { Link } from "react-router";
import { Copyable } from "../ui/Copyable";
import { Icon } from "../ui/Icon";
import { keyProps } from "../ui/keyHint";
import css from "./ArtifactTitleLine.module.css";

type ArtifactTitleLineProps = {
  // The way back: where to, and what it's called.
  back: { to: string; label: string };
  title: ReactNode;
  // What's beside the title: where it stands, then at the line's end an
  // ArtifactTitleRef and the page's Pin.
  children?: ReactNode;
};

// ArtifactTitleLine is the first line of an artifact's head: the way
// back, its title, and what's beside it.
export function ArtifactTitleLine({ back, title, children }: ArtifactTitleLineProps) {
  return (
    <div className={css.line}>
      <Link to={back.to} aria-label={back.label} {...keyProps("esc", back.label)}>
        <Icon name="arrow-left" size={14} />
      </Link>
      <h1>{title}</h1>
      {children}
    </div>
  );
}

// ArtifactTitleRef is what the artifact's named by, at the line's end,
// which copies itself: its ref, or the folder it lives in.
export function ArtifactTitleRef({ text, title }: { text: string; title: string }) {
  return <Copyable text={text} className={css.ref} title={title} />;
}
