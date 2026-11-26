import type { Review } from "../api";
import { exact, stamp } from "../ui/when";
import { verdictWords } from "./words";
import css from "./ReviewEarlier.module.css";
import { Text, textClass } from "../ui/Text";

// ReviewEarlier is your earlier reviews of it, folded under their names
// and verdicts: each its summary and its comments, by kind and line.
export function ReviewEarlier({ reviews }: { reviews: Review[] }) {
  return (
    <details className={css.earlier}>
      <summary className={textClass("dim")}>Reviews: {reviews.map((r) => `${r.id} ${verdictWords[r.verdict!]}`).join(" · ")}</summary>
      {reviews.map((r) => (
        <div key={r.id} className={css.review}>
          <div>
            <strong>{r.id}</strong>{" "}
            <Text dim>
              {verdictWords[r.verdict!]} · {r.submitted && <time title={exact(r.submitted)}>{stamp(r.submitted)}</time>}
            </Text>
          </div>
          {r.summary && <p className={css.summary}>{r.summary}</p>}
          <ul>
            {r.comments.map((n) => (
              <li key={n.id}>
                {n.kind && <span className={`kind k-${n.kind}`}>{n.kind}</span>}{" "}
                <Text mono dim>
                  {n.path}:{n.line}
                </Text>{" "}
                {n.body}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </details>
  );
}
