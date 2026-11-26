import type { ReactNode } from "react";
import { Copyable } from "../ui/Copyable";
import css from "./SpacePreviewHead.module.css";
import { Text } from "../ui/Text";

type SpacePreviewHeadProps = {
  chips?: ReactNode; // its kind and where it stands
  made?: string; // its number or date
  docRef: string; // repo:path, copied
  where: string; // repo · path, as it's shown
  title: string;
  meta?: ReactNode; // who changed it last, how long it is
  actions: ReactNode; // Read it, Edit as a page, In Files
};

// SpacePreviewHead is what the document picked is and where: its kind,
// where it stands and its ref (which copies), its title, who changed it
// last and how long it is, and what's done with it.
export function SpacePreviewHead({ chips, made, docRef, where, title, meta, actions }: SpacePreviewHeadProps) {
  return (
    <header className={css.head}>
      <div className={css.where}>
        {chips}
        {made && <Text mono>{made}</Text>}
        <Copyable text={docRef} className={css.ref} title="Copy its ref">
          {where}
        </Copyable>
      </div>
      <h2 className={css.title}>{title}</h2>
      <p className={css.meta}>{meta}</p>
      <div className={css.actions}>{actions}</div>
    </header>
  );
}
