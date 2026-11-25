import type { Review } from "../api";
import { exact, stamp } from "../ui/when";
import { verdictWords } from "./words";
import css from "./ReviewEarlier.module.css";

// ReviewEarlier is your earlier reviews of it, folded under their names
// and verdicts: each its summary and its comments, by kind and line.
export function ReviewEarlier({ reviews }: { reviews: Review[] }) {
  return (
    <details className={css.earlier}>
      <summary className="dim">Reviews: {reviews.map((r) => `${r.id} ${verdictWords[r.verdict!]}`).join(" · ")}</summary>
      {reviews.map((r) => (
        <div key={r.id} className={css.review}>
          <div>
            <strong>{r.id}</strong>{" "}
            <span className="dim">
              {verdictWords[r.verdict!]} · {r.submitted && <time title={exact(r.submitted)}>{stamp(r.submitted)}</time>}
            </span>
          </div>
          {r.summary && <p className={css.summary}>{r.summary}</p>}
          <ul>
            {r.comments.map((n) => (
              <li key={n.id}>
                {n.kind && <span className={`kind k-${n.kind}`}>{n.kind}</span>}{" "}
                <span className="mono dim">
                  {n.path}:{n.line}
                </span>{" "}
                {n.body}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </details>
  );
}
