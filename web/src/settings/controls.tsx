import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { getModels, type ModelInfo, type Setting } from "../api";
import { useLoaded } from "../load";
import { defaultAccent, parseHex, pulled, showAccent, shownAs } from "../ui/accent";
import { Icon } from "../ui/Icon";
import { toast } from "../ui/kit";
import { Option, Select } from "../ui/Select";
import { Switch } from "../ui/Switch";
import { useTheme } from "../ui/theme";
import { said, shown } from "./sections";
import { Button } from "../ui/Button";
import { Segment, Segmented } from "../ui/Segmented";

// A setting's control, by its kind: what it's set to, and the way to set
// it. Each writes as soon as it's changed (a switch flipped, a field
// left), so there's nothing to save. None waits for another's write: the
// page makes them in turn.

// A control's set writes a value, done when it's written (or refused).
export type Set = (v: unknown) => Promise<void>;

interface Props {
  s: Setting;
  set: Set;
  // reset takes yours away, so the team's or the default applies again.
  reset: () => void;
}

export function Control(p: Props) {
  const { s, set } = p;
  switch (s.kind) {
    case "harness":
      return <Choice {...p} />;
    case "choice":
      return few(s) ? <Few {...p} /> : <Choice {...p} />;
    case "bool":
      return <Switch on={s.value === true} onChange={(on) => void set(on)} aria-label={s.label} />;
    case "model":
      return <ModelInput {...p} />;
    case "picks":
      return <Picks {...p} />;
    case "number":
      return <Stepper {...p} />;
    case "color":
      // The one colour setting is the accent, which the page wears.
      return s.section === "ui" && s.key === "accent" ? <AccentInput {...p} /> : <TextInput {...p} />;
    default:
      return <TextInput {...p} short />;
  }
}

// few is a choice with so few, so briefly said, that they're all on show.
const few = (s: Setting) => !!s.choices && s.choices.length > 1 && s.choices.length <= 3 && s.choices.every((c) => (c.label ?? c.value).length <= 14);

function Choice({ s, set }: Props) {
  return (
    <Select value={shown(s.value)} onChange={set} aria-label={s.label}>
      {!s.choices?.some((c) => c.value === shown(s.value)) && <Option value={shown(s.value)}>{shown(s.value) || "—"}</Option>}
      {s.choices?.map((c) => (
        <Option key={c.value} value={c.value} title={c.note}>
          {`${c.label ?? c.value}${c.note ? " (not here)" : ""}`}
        </Option>
      ))}
    </Select>
  );
}

function Few({ s, set }: Props) {
  return (
    <Segmented value={String(s.value)} onChange={(v) => void set(v)} aria-label={s.label}>
      {s.choices?.map((c) => (
        <Segment key={c.value} value={c.value} title={c.note}>
          {c.label ?? c.value}
        </Segment>
      ))}
    </Segmented>
  );
}

// TextInput commits on Enter or leaving it, if it changed; Escape leaves
// it as it was. blank starts it empty (another model, by name). done is
// told once it's left, whatever came of it.
function TextInput({ s, set, short, placeholder, blank, done }: Props & { short?: boolean; placeholder?: string; blank?: boolean; done?: () => void }) {
  // A model that's the default is said by the placeholder, the field empty.
  const was = blank || (s.source === "default" && s.kind === "model") ? "" : shown(s.value);
  const [text, setText] = useState(was);
  useEffect(() => setText(was), [was]);
  const cancelled = useRef(false);
  const field = useRef<HTMLInputElement>(null);
  // It's come in place of the list it was picked from, which holds the
  // keys until it's closed: they're taken once it has.
  useEffect(() => {
    if (!blank) return;
    const frame = requestAnimationFrame(() => field.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [blank]);
  const commit = () => {
    const t = text.trim();
    if (cancelled.current || t === "" || t === was) setText(was);
    else void set(t);
    cancelled.current = false;
    done?.();
  };
  return (
    <input
      ref={field}
      type="text"
      className={`setting-input${s.kind === "model" ? " mono" : ""}${short ? " short" : ""}`}
      value={text}
      placeholder={placeholder}
      spellCheck={false}
      aria-label={s.label}
      onChange={(e) => setText(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.currentTarget.blur();
        if (e.key === "Escape") {
          cancelled.current = true;
          e.currentTarget.blur();
        }
      }}
    />
  );
}

// Stepper is a whole number: typed, or stepped by one.
function Stepper({ s, set }: Props) {
  const n = typeof s.value === "number" ? s.value : Number(s.value) || 0;
  const was = shown(s.value);
  const [text, setText] = useState(was);
  useEffect(() => setText(was), [was]);
  const cancelled = useRef(false);
  const commit = () => {
    const t = text.trim();
    const cancel = cancelled.current;
    cancelled.current = false;
    if (cancel || t === "" || t === was) return setText(was);
    // What isn't a number would be sent as nothing, which clears the key.
    if (!/^\d+$/.test(t)) {
      setText(was);
      return toast(`${s.label} is a whole number.`, { tone: "danger" });
    }
    void set(Number(t));
  };
  return (
    <span className="setting-stepper">
      <Button type="button" disabled={n <= 1} title="One fewer" aria-label={`${s.label}: one fewer`} onClick={() => void set(n - 1)}>
        <Icon name="minus" size={12} />
      </Button>
      <input
        type="text"
        inputMode="numeric"
        value={text}
        aria-label={s.label}
        onChange={(e) => setText(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
          if (e.key === "Escape") {
            cancelled.current = true;
            e.currentTarget.blur();
          }
        }}
      />
      <Button type="button" title="One more" aria-label={`${s.label}: one more`} onClick={() => void set(n + 1)}>
        <Icon name="plus" size={12} />
      </Button>
    </span>
  );
}

// Picks is a list (of harness:model), each its own: taken off with its ×,
// or Backspace from the empty field; one more added by typing it, then
// Enter, a comma, or leaving the field. Yours can't be an empty list (that
// is no list: the team's, or none), so taking your last one off resets it.
function Picks({ s, set, reset }: Props) {
  const picks = Array.isArray(s.value) ? (s.value as string[]) : [];
  const [text, setText] = useState("");
  const field = useRef<HTMLInputElement>(null);
  const add = () => {
    const more = text
      .split(/[\s,]+/)
      .filter(Boolean)
      .filter((p, i, all) => !picks.includes(p) && all.indexOf(p) === i);
    setText("");
    if (more.length > 0) void set([...picks, ...more]);
  };
  const drop = (i: number) => {
    const left = picks.filter((_, j) => j !== i);
    if (left.length === 0 && s.source === "yours") reset();
    else void set(left);
  };
  return (
    // A press on the box anywhere is a press on its field.
    <div
      className="setting-picks"
      onMouseDown={(e) => {
        if (e.target !== e.currentTarget) return;
        e.preventDefault();
        field.current?.focus();
      }}
    >
      {picks.map((p, i) => (
        <span key={p} className="setting-pick">
          {p}
          <Button type="button" title={`Take ${p} off`} aria-label={`Take ${p} off`} onClick={() => drop(i)}>
            <Icon name="x" size={10} />
          </Button>
        </span>
      ))}
      <input
        ref={field}
        type="text"
        className="mono"
        value={text}
        placeholder={picks.length > 0 ? "add another" : "claude:sonnet"}
        spellCheck={false}
        aria-label={`${s.label}: add one`}
        onChange={(e) => setText(e.target.value)}
        onBlur={add}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault();
            add();
          }
          if (e.key === "Backspace" && text === "" && picks.length > 0) drop(picks.length - 1);
          if (e.key === "Escape") setText("");
        }}
      />
    </div>
  );
}

// What a harness lists is asked of its CLI, which is slow: asked once for
// all the roles on it, while the page is open.
const listed = new Map<string, Promise<ModelInfo[]>>();
function modelsOf(harness: string): Promise<ModelInfo[]> {
  let p = listed.get(harness);
  if (!p) {
    p = getModels(harness);
    listed.set(harness, p);
    p.catch(() => listed.delete(harness));
  }
  return p;
}

// forgetModels has them asked again, the next time: on coming to the page.
export const forgetModels = () => listed.clear();

const own = "\u0000own";
const another = "\u0000another";

// ModelInput is a role's model: one of those its harness lists, or
// another, typed by name (a harness that lists none has the field alone).
function ModelInput(p: Props) {
  const { s, set } = p;
  const harness = s.harness;
  // Another harness's list isn't kept while this one's is asked for: till
  // it's come there's what's set, and another by name.
  const got = useLoaded(harness ? () => modelsOf(harness) : null, [harness]);
  const models = got.value ?? [];
  const asking = !!harness && got.value === undefined && !got.error;
  const [typing, setTyping] = useState(false);
  const cur = shown(s.value);
  const def = shown(s.default);
  const placeholder = def ? `${def} (its default)` : "the agent's own default";
  const note = got.error && (
    <span className="setting-note">
      Couldn't list {harness}'s models: {got.error}
    </span>
  );
  if (typing || (models.length === 0 && !asking)) {
    return (
      <>
        <TextInput {...p} placeholder={typing ? "in that agent's own naming" : placeholder} blank={typing} done={() => setTyping(false)} />
        {note}
      </>
    );
  }
  return (
    <Select
      value={cur || own}
      aria-label={s.label}
      onChange={(v) => {
        if (v === another) setTyping(true);
        else if (v !== own) void set(v);
      }}
    >
      {!cur && <Option value={own}>The agent's own default</Option>}
      {cur && !models.some((m) => m.id === cur) && <Option value={cur}>{cur}</Option>}
      {models.map((m) => (
        <Option key={m.id} value={m.id} note={[m.about ?? (m.name !== m.id ? m.name : ""), m.id === def ? "its default" : ""].filter(Boolean).join(" · ")}>
          {m.id}
        </Option>
      ))}
      <Option value={another}>Another, by name…</Option>
    </Select>
  );
}

// Accents to pick from, beside ws's own and any other: each reads in
// either theme much as it's picked.
const accents: [hex: string, name: string][] = [
  ["#4a5bd0", "Indigo"],
  ["#2b6cb8", "Blue"],
  ["#0e7f86", "Teal"],
  ["#2f7d4f", "Green"],
  ["#a8650f", "Amber"],
  ["#b8456f", "Rose"],
  ["#59626f", "Slate"],
];

// AccentInput is the accent's control: ws's own and a few others to pick
// from, a colour picker for any other, and the hex it's at, any of which
// changes it. The page follows as you pick (the accent on <html>), and the
// pick's written once you let it go; one that isn't kept (refused, or the
// page left first) goes back to what's set. Under them is the accent as
// this theme shows it: in text, as a tint, and on the button. A pick the
// theme lightens or darkens so it reads on the page shows as it's drawn,
// and says so.
function AccentInput({ s, set, reset }: Props) {
  const saved = typeof s.value === "string" ? s.value : null;
  const [ours] = useState(defaultAccent);
  const [preview, setPreview] = useState<string | null>(null);
  const picked = preview ?? saved ?? ours;
  const [editing, setEditing] = useState<string | null>(null);
  const { dark } = useTheme();
  const [themed, setThemed] = useState("");
  const picker = useRef<HTMLInputElement>(null);
  const waiting = useRef<number>(undefined);
  const writing = useRef(false);
  const cancelled = useRef(false);

  // The page wears what's being picked, else what's set, before it paints.
  useLayoutEffect(() => {
    showAccent(preview ?? saved);
    setThemed(shownAs("--accent"));
  }, [preview, saved, dark]);

  // keep writes a pick; the page then wears what's set.
  const keep = (v: string, wait: number) => {
    window.clearTimeout(waiting.current);
    waiting.current = window.setTimeout(() => {
      writing.current = true;
      void set(v).then(() => {
        writing.current = false;
        setPreview(null);
      });
    }, wait);
  };

  // The picker's own change comes when it's let go (React's onChange comes
  // with every move). Safari's can come as it moves, so a pick is written
  // once it's been still a moment.
  const keepRef = useRef(keep);
  keepRef.current = keep;
  useEffect(() => {
    const el = picker.current;
    if (!el) return;
    const changed = () => keepRef.current(el.value, 400);
    el.addEventListener("change", changed);
    return () => el.removeEventListener("change", changed);
  }, []);

  // Left mid-pick, the page goes back to what's set; left while a pick's
  // being written, it keeps the pick.
  const kept = useRef(saved);
  kept.current = saved;
  useEffect(
    () => () => {
      window.clearTimeout(waiting.current);
      if (!writing.current) showAccent(kept.current);
    },
    [],
  );

  // typed writes the hex typed, once it's left: what isn't a hex colour
  // goes to be refused, saying why.
  const typed = (text: string) => {
    setEditing(null);
    const t = text.trim();
    const hex = parseHex(t);
    if (cancelled.current || t === "" || hex === (saved ?? ours)) {
      cancelled.current = false;
      setPreview(null);
      return;
    }
    keep(hex ?? t, 0);
  };

  // ws's own is no accent set: yours taken away, unless the team has one,
  // which only ws's own, set as yours, is over.
  const pickOurs = () => {
    if (s.team !== undefined && s.team !== null) keep(ours, 0);
    else if (s.source === "yours") reset();
  };
  const way = pulled(picked, themed);
  const swatch = (hex: string) => ({ "--swatch": hex }) as CSSProperties;
  const other = picked !== ours && !accents.some(([hex]) => hex === picked);

  return (
    <div className="setting-accent">
      <div className="accent-swatches" role="group" aria-label={`${s.label}: ones to pick from`}>
        <button type="button" className="accent-swatch" style={swatch(ours)} aria-pressed={picked === ours} title="ws's own" aria-label="ws's own" onClick={pickOurs} />
        {accents.map(([hex, label]) => (
          <button key={hex} type="button" className="accent-swatch" style={swatch(hex)} aria-pressed={picked === hex} title={label} aria-label={label} onClick={() => picked !== hex && keep(hex, 0)} />
        ))}
        <label className={`accent-swatch any${other ? " on" : ""}`} style={swatch(picked)} title="Another colour">
          <input ref={picker} type="color" value={picked} aria-label={`${s.label}: another colour`} onChange={(e) => setPreview(e.target.value)} />
        </label>
      </div>
      <div className="accent-line">
        <span className="accent-shown" title={`As the ${dark ? "dark" : "light"} theme shows it`}>
          <span className="accent-shown-text">Text</span>
          <span className="accent-shown-tint">Selected</span>
          <span className="accent-shown-button">Button</span>
        </span>
        <input
          type="text"
          className="mono setting-input"
          value={editing ?? picked}
          spellCheck={false}
          aria-label={`${s.label}, as hex`}
          onFocus={() => setEditing(picked)}
          onChange={(e) => {
            setEditing(e.target.value);
            const hex = parseHex(e.target.value);
            if (hex) setPreview(hex);
          }}
          onBlur={(e) => typed(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") e.currentTarget.blur();
            if (e.key === "Escape") {
              cancelled.current = true;
              e.currentTarget.blur();
            }
          }}
        />
      </div>
      {way && (
        <span className="setting-note">
          The {dark ? "dark" : "light"} theme shows it {way}, as <code>{themed}</code>, so it reads on the page.
        </span>
      )}
    </div>
  );
}

// valueSaid is a setting's value as a line of the page says it, out of
// its control: a switch as On or Off, a choice by its label.
export function valueSaid(s: Setting, v: unknown = s.value): string {
  if (typeof v === "boolean") return v ? "On" : "Off";
  return said(s, v);
}
