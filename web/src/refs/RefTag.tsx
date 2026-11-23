import type { ReactNode } from "react";
import { Link, useNavigate } from "react-router";
import { useHoverDelay } from "../ui/useHoverDelay";
import { Icon } from "../ui/Icon";
import { useWorkspaceIfRead } from "../workspace/store";
import { useRefs } from "./known";
import { RefCard } from "./RefCard";
import { goneCapsule, GROUND, isOutside, type PageRef, refHref, refName, refOf } from "./refs";
import { lookOf } from "./views";

// RefTag is a ref where something's only read (a conversation, a thread,
// a pass, a handoff), drawn as its kind's Look says (views.tsx): its words
// or a pill, with where it stands after them, and its card on hover. A
// click goes there: in ws, or, for a plugin's thing with nowhere to open
// in ws, to its address outside, in a tab of its own. words is a link
// whose words are the writer's own (the auth config), not the thing's name:
// they're kept, with the kind's icon before them.
//
// inline is a line of text (a log line, a list row) that may sit inside a
// link already: the tag is words that go there when clicked, not a link in
// a link, and says nothing after it.
export function RefTag({ refKey, words, inline, children }: { refKey: string; words?: boolean; inline?: boolean; children?: ReactNode }) {
  const workspace = useWorkspaceIfRead();
  const navigate = useNavigate();
  const hover = useHoverDelay<HTMLElement>(160);
  const refs = useRefs();
  const known = refs?.known;
  const r = refOf(refKey);
  if (!r || !workspace) return <>{children}</>;
  const look = lookOf(r, { workspace, repos: refs?.repos ?? new Map(), things: known?.things });
  // A link whose words are the ref's name is drawn as the ref.
  const said = typeof children === "string" ? children : Array.isArray(children) && children.every((c) => typeof c === "string") ? children.join("") : undefined;
  words = words && said !== refName(r);
  // A capsule that's gone: its file's on the default branch now, most likely.
  const to: PageRef = (r.kind === "file" || r.kind === "dir") && goneCapsule(r, known) ? { ...r, worktree: GROUND } : r;
  const href = refHref(to, known);
  const outside = isOutside(href);
  const { label, tone } = (!inline && look.after) || {};
  // Its own text in place of its name, unless the words are the writer's.
  const text = !words && look.text ? look.text : undefined;
  const props = {
    className: `named-ref k-${r.kind} shape-${look.shape}${words ? " ref-words" : ""}`,
    style: look.accent ? ({ "--ref-accent": look.accent } as React.CSSProperties) : undefined,
    title: text ? refName(r) : undefined,
    "data-ref": refKey,
    onMouseEnter: (e: React.MouseEvent<HTMLElement>) => hover.over(e.currentTarget),
    onMouseLeave: hover.out,
    onFocus: (e: React.FocusEvent<HTMLElement>) => hover.over(e.currentTarget),
    onBlur: hover.out,
  };
  const inside = (
    <>
      {(words || look.shape === "pill") && <Icon name={look.icon} size={12} className="ref-icon" />}
      {text ?? children}
    </>
  );
  return (
    <>
      {inline ? (
        <span
          {...props}
          role="link"
          tabIndex={0}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            if (outside) window.open(href, "_blank", "noopener,noreferrer");
            else navigate(href);
          }}
        >
          {inside}
        </span>
      ) : outside ? (
        <a href={href} target="_blank" rel="noreferrer" {...props}>
          {inside}
        </a>
      ) : (
        <Link to={href} {...props}>
          {inside}
        </Link>
      )}
      {label && (
        // Not the text's own words: a reader quoting it leaves this out.
        <span className={`ref-label${tone ? ` tone-${tone}` : ""}`} data-skip="">
          {label}
        </span>
      )}
      {hover.shown && <RefCard r={r} el={hover.shown} onEnter={hover.stay} onLeave={hover.out} hint={false} />}
    </>
  );
}
