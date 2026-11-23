import css from "./WorkspaceMark.module.css";

// WorkspaceMark is the workspace's mark: the first letter of its name on
// the brand's run. It stands for the workspace where there's no room for
// its name: the rail.
export function WorkspaceMark({ name }: { name: string }) {
  return (
    <span aria-hidden className={css.mark}>
      {(name.match(/[\p{L}\p{N}]/u)?.[0] ?? "w").toUpperCase()}
    </span>
  );
}
