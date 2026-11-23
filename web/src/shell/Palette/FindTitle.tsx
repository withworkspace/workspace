import { cx } from "../../ui/cx";
import css from "./FindTitle.module.css";
import type { TitlePart } from "./title";

// FindTitle is what a row of Find's is called, the letters that matched
// what's typed standing out (parts: titleParts makes them). path: it's a
// path, in the code's font, its folders quiet and its name not, and read
// from its end where it's too long to fit.
export function FindTitle({ parts, path }: { parts: TitlePart[]; path?: boolean }) {
  return (
    <span className={cx(css.title, path && css.path)}>
      <bdi>
        {parts.map((p, i) => (
          <span key={i} className={cx(p.hit && css.hit, p.name && css.name)}>
            {p.text}
          </span>
        ))}
      </bdi>
    </span>
  );
}
