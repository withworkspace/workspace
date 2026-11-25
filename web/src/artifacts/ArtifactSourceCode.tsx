import css from "./ArtifactSourceCode.module.css";

// ArtifactSourceCode is a file's lines, numbered: each line is its HTML,
// coloured or escaped by whoever read it.
export function ArtifactSourceCode({ lines }: { lines: string[] }) {
  return (
    <div className={css.code}>
      {lines.map((h, i) => (
        <div key={i} className={css.line}>
          <span className={css.ln}>{i + 1}</span>
          <span className={css.mark} aria-hidden />
          <code dangerouslySetInnerHTML={{ __html: h || " " }} />
        </div>
      ))}
    </div>
  );
}
