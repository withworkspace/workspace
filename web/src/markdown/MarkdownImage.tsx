import { useContext } from "react";
import { rawURL } from "../api";
import { PlaceContext } from "./context";
import { resolveLink } from "./links";
import css from "./Markdown.module.css";

// MarkdownImage is an image in a document: one that's a file in the repo
// is read from there.
export function MarkdownImage({ src, alt, title }: { src?: string; alt?: string; title?: string }) {
  const at = useContext(PlaceContext);
  if (!src) return null;
  const t = resolveLink(at?.path ?? "", src);
  if (t.kind === "external") return <img src={src} alt={alt ?? ""} title={title} loading="lazy" data-viewable={title || alt || ""} />;
  if (!at || t.kind !== "file" || !t.path)
    return (
      <span className={css.deadLink} title="An image this can't find">
        {alt || src}
      </span>
    );
  return <img src={rawURL(at.api, t.path)} alt={alt ?? ""} title={title} loading="lazy" data-viewable={title || alt || t.path} />;
}
