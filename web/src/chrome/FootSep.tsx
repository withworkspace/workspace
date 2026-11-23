import css from "./FootSep.module.css";

// FootSep is a hair between two groups of the foot's parts: the crew, the
// runs, the repos.
export function FootSep() {
  return <span className={css.sep} aria-hidden />;
}
