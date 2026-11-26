import { Button } from "../ui/Button";
import css from "./SpacePreviewSections.module.css";

// SpacePreviewSections is the document picked's sections, each pressed
// to scroll the preview there.
export function SpacePreviewSections({ sections, onGo }: { sections: { anchor: string; text: string }[]; onGo: (anchor: string) => void }) {
  return (
    <nav className={css.sections} aria-label="Sections">
      {sections.map((s) => (
        <Button variant="link" key={s.anchor} onClick={() => onGo(s.anchor)}>
          {s.text}
        </Button>
      ))}
    </nav>
  );
}
