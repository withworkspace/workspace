import { type ReactNode, useEffect, useLayoutEffect, useState, useSyncExternalStore } from "react";
import { Icon, type IconName } from "./Icon";
import { Button } from "./Button";
import { capital } from "./text";
import { getToasts, onToasts, setToasts, toast } from "./toast";

export { toast };

// The small shared pieces every page draws with.

// Keys shows a key or a chord: "g m" is g then m, "mod+⏎" is ⌘⏎.
export function Keys({ k, className }: { k: string; className?: string }) {
  const mac = /Mac|iPhone|iPad/.test(navigator.platform);
  const parts = k.split(" ").map((p) => p.replace(/^mod\+/, mac ? "⌘" : "Ctrl+").replace(/(^|\+|⌘)shift\+/, (_, before: string) => before + (mac ? "⇧" : "Shift+")));
  return (
    <span className={`keys${className ? ` ${className}` : ""}`}>
      {parts.map((p, i) => (
        <kbd key={i}>{p}</kbd>
      ))}
    </span>
  );
}

// Copyable is a ref, path, hash or command that copies itself when
// clicked, and says so.
export function Copyable({ text, children, className, title }: { text: string; children?: ReactNode; className?: string; title?: string }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 1200);
    return () => clearTimeout(t);
  }, [copied]);
  return (
    <button
      type="button"
      className={`copyable${copied ? " copied" : ""}${className ? ` ${className}` : ""}`}
      title={copied ? "Copied" : (title ?? `Copy ${text}`)}
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        navigator.clipboard?.writeText(text).then(() => setCopied(true), () => toast("Couldn't copy: the browser said no"));
      }}
    >
      <span className="copyable-text">{children ?? text}</span>
      <Icon name={copied ? "check" : "copy"} size={12} className="copyable-icon" />
    </button>
  );
}

export type CalloutTone = "attention" | "info" | "danger" | "done" | "quiet" | "progress";

const toneIcon: Record<CalloutTone, IconName> = {
  attention: "alert",
  info: "info",
  danger: "x-circle",
  done: "check-circle",
  quiet: "info",
  progress: "circle-dot",
};

// Callout is a note in the flow of a page: a tinted panel with a bar on
// its left, an icon, a title, and what follows.
export function Callout({
  tone = "info",
  icon,
  title,
  children,
  actions,
  className,
}: {
  tone?: CalloutTone;
  icon?: IconName | null;
  title?: ReactNode;
  children?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  const ic = icon === null ? undefined : (icon ?? toneIcon[tone]);
  return (
    <div className={`callout tone-${tone}${className ? ` ${className}` : ""}`} role={tone === "danger" ? "alert" : undefined}>
      {ic && <Icon name={ic} size={16} className="callout-icon" />}
      <div className="callout-body">
        {title && <div className="callout-title">{title}</div>}
        {children && <div className="callout-text">{children}</div>}
        {actions && <div className="callout-actions">{actions}</div>}
      </div>
    </div>
  );
}

// Empty is a place with nothing in it yet: what it's for, and what to do.
export function Empty({ icon = "inbox", title, children, action }: { icon?: IconName; title: ReactNode; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="empty">
      <Icon name={icon} size={20} className="empty-icon" />
      <div className="empty-title">{title}</div>
      {children && <div className="empty-text">{children}</div>}
      {action && <div className="empty-action">{action}</div>}
    </div>
  );
}

// Loading shows nothing for a moment (most loads are quicker), then a
// spinner and what's loading.
export function Loading({ what, delay = 300 }: { what?: string; delay?: number }) {
  const [shown, setShown] = useState(delay === 0);
  useEffect(() => {
    const t = setTimeout(() => setShown(true), delay);
    return () => clearTimeout(t);
  }, [delay]);
  return (
    <div className={`loading${shown ? " shown" : ""}`} role="status" aria-live="polite">
      {shown && (
        <>
          <Icon name="spinner" size={16} className="spin" />
          <span>{what ?? "Loading…"}</span>
        </>
      )}
    </div>
  );
}

// Skeleton is the shape of what's coming: rows of grey bars.
export function Skeleton({ rows = 4, className }: { rows?: number; className?: string }) {
  return (
    <div className={`skeleton${className ? ` ${className}` : ""}`} aria-hidden>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="skeleton-row" style={{ width: `${[92, 76, 84, 60, 88, 70][i % 6]}%` }} />
      ))}
    </div>
  );
}

// ErrorCard is something that didn't load, in words, with a retry.
export function ErrorCard({ error, retry, title = "Couldn't load this" }: { error: string; retry?: () => void; title?: string }) {
  return (
    <Callout tone="danger" title={title} actions={retry && <Button onClick={retry}>Try again</Button>}>
      {humanError(error)}
    </Callout>
  );
}

// humanError makes a server's error a sentence: capitalised, ending in a
// full stop.
export function humanError(e: string): string {
  const s = e.trim().replace(/^error:\s*/i, "");
  if (!s) return "Something went wrong.";
  const t = capital(s);
  return /[.!?)]$/.test(t) ? t : t + ".";
}

// Toasts: what an action did, for a few seconds, bottom centre, with an
// action of its own (Undo, Open) when there's one.

export function Toasts() {
  const list = useSyncExternalStore(onToasts, getToasts);
  return (
    <div className="toasts" role="status" aria-live="polite" data-react-aria-top-layer="">
      {list.map((t) => (
        <div key={t.id} className={`toast tone-${t.tone}`}>
          <Icon name={t.tone === "danger" ? "x-circle" : t.tone === "done" ? "check-circle" : "info"} size={14} />
          <span>{t.text}</span>
          {t.action && (
            <Button
              variant="link"
              className="toast-action"
              onClick={() => {
                t.action!.run();
                setToasts(getToasts().filter((x) => x.id !== t.id));
              }}
            >
              {t.action.label}
            </Button>
          )}
        </div>
      ))}
    </div>
  );
}

// useTitle names the browser tab after the page.
let workspaceName = "ws";
let waitingCount = 0;
let pageTitle = "";

function applyTitle() {
  const parts = [pageTitle, workspaceName].filter(Boolean);
  document.title = (waitingCount ? `(${waitingCount}) ` : "") + parts.join(" · ");
}

export function setTitleContext(name: string, waiting: number) {
  workspaceName = name || "ws";
  waitingCount = waiting;
  applyTitle();
}

export function useTitle(title: string | undefined) {
  useEffect(() => {
    pageTitle = title ?? "";
    applyTitle();
  }, [title]);
}

// fitText sizes a textarea to what's in it, up to max pixels. One that
// isn't laid out (in a view that's hidden) has no height to go by: it's
// left at its rows, and fitted when it's shown.
function fitText(el: HTMLTextAreaElement, max: number) {
  if (el.getClientRects().length === 0) {
    el.style.height = "";
    return;
  }
  el.style.height = "auto";
  el.style.height = `${Math.min(el.scrollHeight + 2, max)}px`;
  el.style.overflowY = el.scrollHeight + 2 > max ? "auto" : "hidden";
}

// useAutosize grows a textarea with what's typed, up to max pixels, and
// fits it again when its width changes: when a hidden view it's in is
// shown, or the window's resized and its lines wrap differently.
export function useAutosize(ref: React.RefObject<HTMLTextAreaElement | null>, value: string, max = 320) {
  useLayoutEffect(() => {
    const el = ref.current;
    if (el) fitText(el, max);
  }, [ref, value, max]);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    // Only its width: fitting it changes its height, which would come
    // back here.
    let width = el.clientWidth;
    const watch = new ResizeObserver(() => {
      if (el.clientWidth === width) return;
      width = el.clientWidth;
      fitText(el, max);
    });
    watch.observe(el);
    return () => watch.disconnect();
  }, [ref, max]);
}

// useMedia is whether a media query matches now, kept up to date.
export function useMedia(query: string): boolean {
  const [on, setOn] = useState(() => window.matchMedia(query).matches);
  useEffect(() => {
    const m = window.matchMedia(query);
    const changed = () => setOn(m.matches);
    m.addEventListener("change", changed);
    changed();
    return () => m.removeEventListener("change", changed);
  }, [query]);
  return on;
}

// useDraft keeps what's typed in a composer across visits, per key, in
// this browser.
export function useDraft(key: string): [string, (v: string) => void] {
  const storeKey = `ws-draft:${key}`;
  const load = () => {
    try {
      return localStorage.getItem(storeKey) ?? "";
    } catch {
      return "";
    }
  };
  const [state, setState] = useState(() => ({ key: storeKey, value: load() }));
  // Another conversation in the same composer: its own draft.
  const value = state.key === storeKey ? state.value : load();
  if (state.key !== storeKey) setState({ key: storeKey, value });
  const set = (v: string) => {
    setState({ key: storeKey, value: v });
    try {
      if (v) localStorage.setItem(storeKey, v);
      else localStorage.removeItem(storeKey);
    } catch {
      // storage off: the draft lives as long as the page
    }
  };
  return [value, set];
}
