import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef, useState } from "react";
import { crewScreenURL, type TermKey, type TermScreen, typeIntoCrew, useVisible } from "../api";

// CrewTerminal is a running crew's terminal: its screen as the server
// emulates it (frames from the server, not the raw stream, so nothing depends
// on joining halfway), and what you type, sent key by key for the server
// to encode for the modes the crew has on. It's drawn at the crew's size,
// with the font scaled to fit: the size is the crew's.
// Loaded only when a crew's page is opened.

export type TermState = "connecting" | "live" | "ended" | "gone";

const modifierKeys = new Set(["Shift", "Control", "Alt", "Meta", "CapsLock", "Fn"]);

export default function CrewTerminal({ slug, id, onState, onTitle }: { slug: string; id: string; onState: (s: TermState) => void; onTitle: (t: string) => void }) {
  const host = useRef<HTMLDivElement>(null);
  const term = useRef<Terminal>(null);
  const fit = useRef<() => void>(() => {});
  const seen = useRef(false); // a screen has come: the crew was running
  const ended = useRef(false);
  const [error, setError] = useState<string>();
  const visible = useVisible();

  // The terminal, and what's typed into it.
  useEffect(() => {
    const el = host.current!;
    const t = new Terminal({
      cursorBlink: true,
      scrollback: 0,
      fontFamily: 'ui-monospace, "SF Mono", Menlo, monospace',
      fontSize: 13,
      theme: { background: "#101012", foreground: "#e6e6ea", cursor: "#e6e6ea" },
    });
    t.open(el);
    term.current = t;
    seen.current = ended.current = false;

    // One input at a time, in order.
    let chain: Promise<unknown> = Promise.resolve();
    const send = (input: { key: TermKey } | { paste: string }) => {
      chain = chain
        .then(() => typeIntoCrew(slug, id, input))
        .then(() => setError(undefined))
        .catch((e: Error) => setError(e.message));
    };
    t.attachCustomKeyEventHandler((ev) => {
      if (ev.type !== "keydown") return false;
      if (ev.metaKey) return true; // ⌘ is the browser's: copy, paste, …
      if (ev.isComposing || modifierKeys.has(ev.key)) return false;
      let key = ev.key;
      // Option on a Mac makes ∫ of b: the crew wants alt+b.
      if (ev.altKey && /^(Key[A-Z]|Digit[0-9])$/.test(ev.code)) key = ev.code.slice(-1).toLowerCase();
      ev.preventDefault(); // the page's own keys (Escape, ?) stay out of it
      send({ key: { key, ctrl: ev.ctrlKey, alt: ev.altKey, shift: ev.shiftKey } });
      return false;
    });
    // What the keys above don't send: pastes, and composed text.
    t.onData((data) => send({ paste: data }));

    fit.current = () => {
      const size = Math.max(8, Math.min(14, Math.floor(el.clientWidth / (t.cols * 0.62))));
      if (t.options.fontSize !== size) t.options.fontSize = size;
    };
    const onResize = () => fit.current();
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      term.current = null;
      t.dispose();
    };
  }, [slug, id]);

  // Its screen, streamed while the page is showing; showing again catches
  // up with a fresh screen.
  useEffect(() => {
    if (!visible || ended.current) return;
    const es = new EventSource(crewScreenURL(slug, id));
    es.addEventListener("screen", (e) => {
      const t = term.current;
      if (!t) return;
      const sc = JSON.parse((e as MessageEvent).data) as TermScreen;
      if (t.cols !== sc.w || t.rows !== sc.h) t.resize(sc.w, sc.h);
      draw(t, sc);
      fit.current();
      if (!seen.current) t.focus();
      seen.current = true;
      onState("live");
      onTitle(sc.title ?? "");
    });
    es.addEventListener("exit", () => {
      es.close();
      ended.current = true;
      onState("ended");
    });
    es.onerror = () => {
      if (es.readyState === EventSource.CLOSED) {
        ended.current = true;
        onState(seen.current ? "ended" : "gone");
      }
    };
    return () => es.close();
  }, [slug, id, visible, onState, onTitle]);

  return (
    <>
      <div ref={host} className="crew-term" />
      {error && <div className="term-error">Not sent: {error}</div>}
    </>
  );
}

// draw paints a screen: each row erased and written at its place, then the
// cursor where the crew's is.
function draw(term: Terminal, sc: TermScreen) {
  const lines = sc.text.split("\n");
  let out = "\x1b[?25l";
  for (let i = 0; i < sc.h; i++) {
    out += `\x1b[${i + 1};1H\x1b[0m\x1b[2K${lines[i] ?? ""}`;
  }
  out += `\x1b[0m\x1b[${sc.cy + 1};${sc.cx + 1}H`;
  if (sc.cv) out += "\x1b[?25h";
  term.write(out);
}
