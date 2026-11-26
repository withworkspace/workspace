import { Copyable } from "../../ui/Copyable";
import css from "./ReaderFilePath.module.css";

// ReaderFilePath is where a document's file is, at its foot, to copy.
export function ReaderFilePath({ path }: { path: string }) {
  return (
    <p className={css.path}>
      <Copyable text={path} title="Copy path" />
    </p>
  );
}
