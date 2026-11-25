import css from "./ReviewCommon.module.css";

// ReviewCommon is what every file to read is under, said once over them.
export function ReviewCommon({ path }: { path: string }) {
  return (
    <div className={css.common} title={`Every file to read is under ${path}`}>
      <bdi>{path}</bdi>
    </div>
  );
}
