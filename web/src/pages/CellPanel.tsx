import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { EditorView } from "prosemirror-view";
import type { PageCell } from "../api";
import { cellElement, dropCell, fillCell, keepCell } from "../editor/cells";
import { Markdown } from "../markdown/LazyMarkdown";
import { Button } from "../ui/Button";
import { Calls } from "../ui/Calls";
import { Icon } from "../ui/Icon";
import { SendKeys } from "../ui/SendKeys";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import { toast } from "../ui/toast";
import { humanError } from "../ui/strings";
import { Turn, TurnSaid } from "../ui/Turn";
import { askingNow, baseName, besideChanged, changedSources, liveCalls, outOfDate, sourceHref, sourceName, versions } from "./cell";
import { PageComposer } from "./PageComposer";
import { PageNotes } from "./PageNotes";
import { PagePanel } from "./PagePanel";
import { PagePanelFoot } from "./PagePanelFoot";
import { PagePanelHead } from "./PagePanelHead";
import { PageSourceChip } from "./PageSourceChip";
import { PageSources } from "./PageSources";
import { PageStale } from "./PageStale";
import { PageTurn } from "./PageTurn";
import { PageTurns } from "./PageTurns";
import { PageUnstarted } from "./PageUnstarted";
import { PageVersion } from "./PageVersion";
import { textClass } from "../ui/Text";
import { RichText } from "../ui/RichText";

// CellPanel is a cell opened: what was asked for, each version and what
// you asked to change between them, what the latest was written from, and
// a box to ask for a change. It sits under the cell's head.
export function CellPanel({
  cell,
  view,
  wrap,
  root,
  onClose,
  onSay,
  onRetry,
}: {
  cell: PageCell;
  view: EditorView;
  wrap: React.RefObject<HTMLDivElement | null>;
  root: string;
  onClose: () => void;
  onSay: (body: string, ask?: boolean) => Promise<void>;
  onRetry: () => Promise<void>;
}) {
  const panel = useRef<HTMLDivElement>(null);
  const box = useRef<HTMLElement>(null);
  const sendKey = useSendKey();
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [at, setAt] = useState<{ top: number; left: number }>();

  // Under the cell, so what it holds shows while you ask for a change; a
  // cell too long for that has it under its head. Wherever that now is.
  useLayoutEffect(() => {
    const place = () => {
      const el = cellElement(view, cell.id);
      const head = el?.querySelector(".cell-head");
      const host = wrap.current;
      if (!el || !head || !host) return onClose(); // the cell's gone from the document
      const c = el.getBoundingClientRect();
      const r = c.height < window.innerHeight * 0.5 ? c : head.getBoundingClientRect();
      const w = host.getBoundingClientRect();
      const width = panel.current?.offsetWidth ?? 440;
      setAt({ top: r.bottom - w.top + 6, left: Math.max(0, Math.min(c.left - w.left, w.width - width)) });
    };
    place();
    window.addEventListener("resize", place);
    return () => window.removeEventListener("resize", place);
    // Placed again as the cell changes above or around it.
  }, [view, cell, wrap, onClose]);

  // Opened, it's ready for what you want changed.
  const placed = !!at;
  useEffect(() => {
    if (placed) box.current?.focus();
  }, [placed]);

  useEffect(() => {
    const down = (e: MouseEvent) => {
      const t = e.target as Node;
      if (panel.current?.contains(t) || cellElement(view, cell.id)?.querySelector(".cell-head")?.contains(t)) return;
      onClose();
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        onClose();
      }
    };
    document.addEventListener("mousedown", down);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("mousedown", down);
      document.removeEventListener("keydown", key, true);
    };
  }, [view, cell.id, onClose]);

  const run = (what: () => Promise<void>) => {
    setBusy(true);
    what()
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const all = versions(cell);
  const latest = all.at(-1);
  // A question with nothing written yet asks first, and can be told to put
  // it in the page; a cell is changed first, and can just be asked.
  const question = !latest && !!cell.turns[0]?.ask;
  const send = (ask: boolean) => {
    const body = text.trim() || (question && !ask ? "Write that up for the page, here." : "");
    if (!body || busy) return;
    run(() => onSay(body, ask).then(() => setText("")));
  };
  const read = cell.sources ?? [];
  const moved = changedSources(cell);
  const stale = outOfDate(cell);
  // Written again from what's there now: it's asked to, naming what moved.
  const again = () =>
    run(() =>
      onSay(
        moved.length
          ? `Since you wrote this, ${moved.map((m) => `${m.path} ${m.state === "gone" ? "has gone" : "has changed"}`).join(", ")}. Read what's there now, and write the cell again from it.`
          : `${besideChanged(cell)} since you wrote this. Read the page again, and write the cell again from what's there now.`,
      ),
    );
  const last = cell.turns.at(-1);
  const unstarted = !cell.answering && last?.by !== "agent";
  return (
    <PagePanel ref={panel} at={at} label="This cell">
      <PagePanelHead prompt={cell.turns[0]?.body} onClose={onClose} />
      {cell.turns.length > 1 || cell.answering || cell.turns[0]?.edited ? (
        <PageTurns>
          {cell.turns[0]?.edited && (
            <PageTurn kind="version">
              <PageVersion>
                <Icon name="pencil" size={11} />
                <span>Your original text</span>
                <Button variant="link" onClick={() => fillCell(view, cell.id, 0, cell.turns[0].edited!)} title="Restore your text">
                  Use this
                </Button>
              </PageVersion>
            </PageTurn>
          )}
          {cell.turns.slice(1).map((t) => {
            if (t.by === "you") {
              return (
                <PageTurn key={t.n}>
                  <Turn by="you" dense landed>
                    <TurnSaid>
                      <RichText text={t.body} />
                    </TurnSaid>
                  </Turn>
                  {t.edited && (
                    <PageVersion yours>
                      <Icon name="pencil" size={11} />
                      <span>With your edits</span>
                      <Button variant="link" onClick={() => fillCell(view, cell.id, 0, t.edited!)} title="Restore your edits">
                        Use this
                      </Button>
                    </PageVersion>
                  )}
                </PageTurn>
              );
            }
            if (t.by !== "agent") {
              return (
                <PageTurn key={t.n} kind="failed">
                  <Icon name="alert" size={12} /> {t.body}
                </PageTurn>
              );
            }
            if (t.answer) {
              return (
                <PageTurn key={t.n} kind="answer">
                  <Turn by="agent" dense landed>
                    <TurnSaid>
                      <Markdown text={t.body} pinned={t.refs} />
                    </TurnSaid>
                    <Calls calls={t.calls ?? []} root={root} />
                  </Turn>
                </PageTurn>
              );
            }
            const k = all.indexOf(t) + 1;
            return (
              <PageTurn key={t.n} kind="version">
                <PageVersion>
                  <Icon name="sparkle" size={12} />
                  <span>{all.length > 1 ? `Version ${k}` : "Written"}</span>
                  <Time iso={t.at} as="clock" className={textClass("dim")} />
                  {t !== latest && (
                    <Button variant="link" onClick={() => fillCell(view, cell.id, k, t.body)} title="Restore this version">
                      Use this
                    </Button>
                  )}
                </PageVersion>
                <Calls calls={t.calls ?? []} root={root} />
                {t.notes && (
                  <PageNotes old={t !== latest}>
                    <Markdown text={t.notes} pinned={t.refs} />
                  </PageNotes>
                )}
              </PageTurn>
            );
          })}
          {cell.answering && (
            <PageTurn kind="filling">
              <PageVersion>
                <Spinner size={12} />
                <span>{askingNow(cell) ? "Answering" : all.length ? "Rewriting" : "Filling in"}</span>
              </PageVersion>
              <Calls calls={liveCalls(cell.run)} root={root} open live />
            </PageTurn>
          )}
        </PageTurns>
      ) : null}
      {read.length > 0 && !cell.answering && (
        <PageSources>
          {read.map((src) => {
            const name = sourceName(src);
            const href = sourceHref(src);
            const said = src.state === "gone" ? `${name}: deleted since written` : src.state ? `${name}: changed since written` : name;
            return (
              <PageSourceChip key={src.path} to={href} moved={!!src.state} title={href ? `${said}. Open in Files.` : said}>
                <Icon name={src.state ? "alert" : "file"} size={11} />
                {name.length > 40 ? `${name.split("/")[0]}/…/${baseName(src)}` : name}
              </PageSourceChip>
            );
          })}
        </PageSources>
      )}
      {stale && !cell.answering && !unstarted && (
        <PageStale words={`${stale} since this was written.`}>
          <Button size="small" disabled={busy} onClick={again}>
            Rewrite
          </Button>
        </PageStale>
      )}
      {unstarted && (
        <PageUnstarted words={last?.by === "ws" ? (askingNow(cell) ? "Could not answer." : "Could not fill.") : "Agent not started."}>
          <Button size="small" disabled={busy} onClick={() => run(onRetry)}>
            Try again
          </Button>
        </PageUnstarted>
      )}
      <PageComposer
        field={box}
        value={text}
        onChange={setText}
        // The key that sends is the first of the two; its other way,
        // the other.
        onSend={({ other }) => send(other ? !question : question)}
        tallest={160}
        placeholder={
          question
            ? "Ask more, or say what to put in the page…"
            : latest
              ? "Say what to change: group them by service, drop the last column… Or ask about it."
              : "Add to what you asked for…"
        }
        aria-label={question ? "Ask or add to page" : "Change or ask about cell"}
        escape="leave"
      >
        {question ? (
          <>
            <Button size="small" disabled={busy} onClick={() => send(false)} title="Write into the page" keys={sendKeys(sendKey, true)}>
              Add to page
            </Button>
            <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={() => send(true)} title="Ask without changing the page" keys={sendKeys(sendKey)}>
              Ask <SendKeys />
            </Button>
          </>
        ) : (
          <>
            <Button size="small" disabled={!text.trim() || busy} onClick={() => send(true)} title="Ask without changing the cell" keys={sendKeys(sendKey, true)}>
              Just ask
            </Button>
            <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={() => send(false)} title="Rewrite the cell this way" keys={sendKeys(sendKey)}>
              Change <SendKeys />
            </Button>
          </>
        )}
      </PageComposer>
      <PagePanelFoot id={cell.id}>
        {!question && (
          <Button
            variant="link"
            title="Keep content as plain text"
            onClick={() => {
              keepCell(view, cell.id);
              onClose();
            }}
          >
            Keep as text
          </Button>
        )}
        <Button
          variant="link"
          title={question ? "Remove question" : "Remove cell and its content"}
          onClick={() => {
            dropCell(view, cell.id);
            onClose();
          }}
        >
          Remove
        </Button>
      </PagePanelFoot>
    </PagePanel>
  );
}
