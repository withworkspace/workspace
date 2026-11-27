import { useContext } from "react";
import { rawURL } from "../api";
import { pictureURL } from "../pictures";
import { PlaceContext } from "./context";
import { resolveLink } from "./links";
import css from "./Markdown.module.css";

// MarkdownImage is an image in a document: one that's a file in the repo
// is read from there, and one pasted for an agent from ws.
export function MarkdownImage({ src, alt, title }: { src?: string; alt?: string; title?: string }) {
  const at = useContext(PlaceContext);
  if (!src) return null;
  // A picture pasted for an agent, named by its path: drawn from ws.
  const pasted = pictureURL(src);
  if (pasted) return <img src={pasted} alt={alt ?? ""} title={title} loading="lazy" className={css.pasted} data-viewable={title || alt || "Picture"} />;
  const t = resolveLink(at?.path ?? "", src);
  if (t.kind === "external") return <img src={src} alt={alt ?? ""} title={title} loading="lazy" data-viewable={title || alt || ""} />;
  if (!at || t.kind !== "file" || !t.path)
    return (
      <span className={css.deadLink} title="Image not found">
        {alt || src}
      </span>
    );
  return <img src={rawURL(at.api, t.path)} alt={alt ?? ""} title={title} loading="lazy" data-viewable={title || alt || t.path} />;
}
