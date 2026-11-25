import { CodeLines } from "../ui/CodeLines";
import css from "./ArtifactSourceCode.module.css";

// ArtifactSourceCode is a file's lines, numbered (ui/CodeLines draws
// them), in a card on the page's own ground: each line is its HTML,
// coloured or escaped by whoever read it. path is the file's, which names
// the lines.
export function ArtifactSourceCode({ lines, path }: { lines: string[]; path: string }) {
  return (
    <div className={css.code}>
      <CodeLines className={css.lines} html={lines} path={path} />
    </div>
  );
}
