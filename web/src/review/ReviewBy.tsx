import css from "./ReviewBy.module.css";

// ReviewBy is who a review's change is by, in its bar.
export function ReviewBy({ name }: { name: string }) {
  return (
    <span className={css.by}>
      by <strong>{name}</strong>
    </span>
  );
}
