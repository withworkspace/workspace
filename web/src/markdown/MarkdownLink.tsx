import { type ReactNode, useContext } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import { BoxContext, PlaceContext, toHeading } from "./context";
import { resolveLink } from "./links";
import css from "./Markdown.module.css";

// MarkdownLink is a link in a document. One to another file opens it in
// the same worktree, where the document says where it is; one to a heading
// goes there; one that leaves the repo has nowhere to go.
export function MarkdownLink({ href, children }: { href?: string; children?: ReactNode }) {
  const at = useContext(PlaceContext);
  const box = useContext(BoxContext);
  const location = useLocation();
  const navigate = useNavigate();
  if (!href) return <a>{children}</a>;
  const t = resolveLink(at?.path ?? "", href);
  if (t.kind === "external") {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer">
        {children}
      </a>
    );
  }
  if (t.kind === "heading") {
    return (
      <a
        href={`#${t.id}`}
        onClick={(e) => {
          if (e.metaKey || e.ctrlKey || e.shiftKey) return;
          e.preventDefault();
          const found = toHeading(box?.current ?? document.body, t.id);
          // The address names the heading, for a document that has one.
          if (found && at) navigate({ search: location.search, hash: `#${t.id}` }, { replace: true, state: { clicked: true } });
        }}
      >
        {children}
      </a>
    );
  }
  // Read without knowing where it is (a reply, a brief), a path is only words.
  if (!at) return <a href={href}>{children}</a>;
  if (t.kind === "outside" || !t.path) {
    return (
      <span className={css.deadLink} title="This leads outside the repo">
        {children}
      </span>
    );
  }
  return <Link to={at.to(t.path) + (t.hash ? `#${t.hash}` : "")}>{children}</Link>;
}
