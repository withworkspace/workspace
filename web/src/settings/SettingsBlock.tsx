import type { ReactNode } from "react";
import { GroupHead } from "../ui/GroupHead";
import { SettingsAbout } from "./SettingsAbout";
import css from "./SettingsBlock.module.css";

type SettingsBlockProps = {
  // What an address's hash names (/settings/missions#orchestrator).
  id?: string;
  // Its heading, and how many it has.
  title?: ReactNode;
  count?: ReactNode;
  // What it's about, under its heading.
  about?: ReactNode;
  children?: ReactNode;
};

// SettingsBlock is a block of a section of the Settings page: its heading,
// what it says, and its rows or whatever else it holds.
export function SettingsBlock({ id, title, count, about, children }: SettingsBlockProps) {
  return (
    <section id={id} className={css.block}>
      {title && (
        <GroupHead className={css.head} count={count}>
          {title}
        </GroupHead>
      )}
      {about && <SettingsAbout>{about}</SettingsAbout>}
      {children}
    </section>
  );
}
