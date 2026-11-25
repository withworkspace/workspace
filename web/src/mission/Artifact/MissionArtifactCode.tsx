import css from "./MissionArtifactCode.module.css";

// MissionArtifactCode is one of a mission's files that isn't a document,
// as code: highlighted (html), on a well, in the code's font.
export function MissionArtifactCode({ html }: { html: string }) {
  return (
    <pre className={css.code}>
      <code dangerouslySetInnerHTML={{ __html: html }} />
    </pre>
  );
}
