import { type ReactNode, useCallback, useEffect, useId, useRef, useState } from "react";
import { Button as AriaButton, Dialog as AriaDialog, DialogTrigger, Popover } from "react-aria-components";
import { Icon } from "../ui/Icon";
import { Copyable, Keys } from "../ui/kit";
import { Option, Select } from "../ui/Select";
import { Switch } from "../ui/Switch";
import { changed, presetOn, readChange, readTold, said, type State, stateWords, takes, type Told, type Tweak, type Value } from "./tweaks";
import { Chip } from "../ui/Chip";
import { Count } from "../ui/Count";
import { Button } from "../ui/Button";

// Tweaks: what a version's page says can be changed about it, drawn
// here, beside its frame, and not by the page: a tray of its presets and
// a control for each tweak. The page declares them with ws.tweaks (which
// the server writes into every frame), tells this page over postMessage,
// and is told back what to set. The frame's origin is its own, so its
// messages are known by the window they come from, and read as data
// (tweaks.ts).

// FrameTweaks is a frame's tweaks, for the page around it: frame is the
// iframe's ref, told what its page declared and the state it's in (none
// until it says, and for a page that declares none), and set changes some
// of them.
export interface FrameTweaks {
  frame: (el: HTMLIFrameElement | null) => void;
  told: Told | undefined;
  set: (some: State) => void;
}

// useTweaks is the tweaks of the frame that's showing what: a version, at
// a run of it (a3.v2#0). What a frame said is kept with what it was
// showing, so the moment the page asks for another (a version stepped to,
// a reload), there's nothing told: not the last frame's, a render late.
export function useTweaks(what: string): FrameTweaks {
  const [el, setEl] = useState<HTMLIFrameElement | null>(null);
  const [said, setSaid] = useState<{ what: string; told: Told }>();
  const showing = useRef(what);
  showing.current = what;
  useEffect(() => {
    if (!el) return;
    const onMessage = (e: MessageEvent) => {
      if (e.source !== el.contentWindow) return;
      const all = readTold(e.data);
      if (all) return setSaid({ what: showing.current, told: all });
      setSaid((s) => {
        const state = s && s.what === showing.current ? readChange(s.told, e.data) : undefined;
        return s && state ? { what: s.what, told: { ...s.told, state } } : s;
      });
    };
    // The page says what it declares as it loads. Asked too, now and once
    // it's loaded, for one that said so before this was listening.
    const hello = () => el.contentWindow?.postMessage({ ws: "tweaks:hello" }, "*");
    window.addEventListener("message", onMessage);
    el.addEventListener("load", hello);
    hello();
    return () => {
      window.removeEventListener("message", onMessage);
      el.removeEventListener("load", hello);
    };
  }, [el]);
  const set = useCallback((some: State) => el?.contentWindow?.postMessage({ ws: "tweaks:set", state: some }, "*"), [el]);
  return { frame: setEl, told: said?.what === what ? said.told : undefined, set };
}

// hasTweaks is whether a frame's page declared any.
export const hasTweaks = (told: Told | undefined): told is Told => !!told && told.tweaks.length > 0;

// TweaksTray is a frame's tweaks, beside it: the artifact's page.
export function TweaksTray({ told, set, onClose, children }: { told: Told; set: (some: State) => void; onClose: () => void; children?: ReactNode }) {
  return (
    <aside className="tweaks-tray" aria-label="Tweaks">
      <TweaksHead told={told}>
        <Button variant="ghost" size="small" square onClick={onClose} aria-label="Hide the tweaks" title="Hide them (t)">
          <Icon name="x" size={13} />
        </Button>
      </TweaksHead>
      <TweaksBody told={told} set={set} />
      {children && <div className="tweaks-foot">{children}</div>}
    </aside>
  );
}

// TweaksButton is a frame's tweaks behind a button, where there isn't the
// room for a tray: a side of a comparison. Nothing, for a page with none.
export function TweaksButton({ told, set, label }: { told: Told | undefined; set: (some: State) => void; label: string }) {
  if (!hasTweaks(told)) return null;
  const n = changed(told).length;
  // data-field, on what it opens: keys typed there are its own, not the
  // page's (keys.ts). Not on the button, which has the focus again once
  // it's shut, when the page's keys should work.
  return (
    <DialogTrigger>
      <AriaButton className="ghost small tweaks-button" aria-label={label}>
        <Icon name="sliders" size={13} /> Tweaks
        {n ? <Count>{n}</Count> : null}
      </AriaButton>
      <Popover className="select-popover tweaks-popover" placement="bottom end" offset={4} data-field>
        <AriaDialog className="tweaks-dialog" aria-label={label}>
          <TweaksHead told={told} />
          <TweaksBody told={told} set={set} />
        </AriaDialog>
      </Popover>
    </DialogTrigger>
  );
}

function TweaksHead({ told, children }: { told: Told; children?: ReactNode }) {
  const n = changed(told).length;
  return (
    <header className="tweaks-head">
      <h2>Tweaks</h2>
      {n ? <Chip>{n} changed</Chip> : null}
      <span className="tweaks-head-end">
        <Copyable text={stateWords(told)} className="tweaks-copy" title="Copy what each is set to, as text: to say which you'd keep">
          Copy
        </Copyable>
        {children}
      </span>
    </header>
  );
}

// TweaksBody is the presets, a button each, and the tweaks, a control
// each. A preset is on while every tweak it lists is as it gives it.
function TweaksBody({ told, set }: { told: Told; set: (some: State) => void }) {
  return (
    <div className="tweaks-body">
      {told.presets.length > 0 && (
        <div className="tweak-presets" role="group" aria-label="Presets">
          {told.presets.map((p, i) => (
            <button key={i} type="button" className="tweak-preset" aria-pressed={presetOn(p, told.state)} onClick={() => set(p.state)}>
              <span className="tweak-preset-label">{p.label}</span>
              {p.description && <span className="tweak-preset-description">{p.description}</span>}
            </button>
          ))}
        </div>
      )}
      <div className="tweak-list">
        {told.tweaks.map((t) => (
          <TweakRow key={t.name} t={t} value={told.state[t.name]} opens={told.opening[t.name]} set={(v) => set({ [t.name]: v })} />
        ))}
      </div>
      {told.problems.length > 0 && (
        <details className="tweak-problems">
          <summary>
            <Icon name="alert" size={12} /> {told.problems.length === 1 ? "A problem" : `${told.problems.length} problems`} with what the page declares
          </summary>
          <ul>
            {told.problems.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}

// short is a choice with few enough options, and short enough, to be a
// row of buttons: otherwise it's a dropdown.
const short = (t: Tweak) => !!t.options && t.options.length <= 4 && t.options.reduce((n, o) => n + o.label.length, 0) <= 26;

// TweakRow is one tweak: its label, what it's for, and its control. A
// switch and a colour sit at the end of the label's line, the rest under
// it. One that isn't as the page opens it is marked, and says how it
// opens.
function TweakRow({ t, value, opens, set }: { t: Tweak; value: Value; opens: Value; set: (v: Value) => void }) {
  const id = useId();
  const inline = t.kind === "toggle" || t.kind === "color";
  const moved = value !== opens;
  const control = <Control id={id} t={t} value={value} set={set} />;
  const chosen = t.kind === "choice" ? t.options?.find((o) => o.value === value)?.description : "";
  return (
    <div className={`tweak${moved ? " moved" : ""}`} data-kind={t.kind}>
      <div className="tweak-line">
        <label htmlFor={id} className="tweak-label">
          {t.label}
        </label>
        {moved && <span className="tweak-moved" title={`It opens as ${said(t, opens)}`} aria-label={`Changed: it opens as ${said(t, opens)}`} />}
        {inline && <span className="tweak-inline">{control}</span>}
      </div>
      {t.description && <p className="tweak-description">{t.description}</p>}
      {!inline && control}
      {chosen && <p className="tweak-description">{chosen}</p>}
    </div>
  );
}

// useShown is what a control shows: what was last put in it, at once, and
// the tweak's value whenever the frame says another (a preset, the page
// itself). The frame's answer is a message away, which a slider being
// dragged or a word being typed can't wait for.
function useShown<T>(value: T): [T, (v: T) => void] {
  const [shown, setShown] = useState(value);
  useEffect(() => setShown(value), [value]);
  return [shown, setShown];
}

function Control({ id, t, value, set }: { id: string; t: Tweak; value: Value; set: (v: Value) => void }) {
  switch (t.kind) {
    case "toggle":
      return <Toggle id={id} t={t} value={value === true} set={set} />;
    case "choice":
      return <Choose id={id} t={t} value={String(value)} set={set} />;
    case "number":
      return <NumberControl id={id} t={t} value={Number(value)} set={set} />;
    case "color":
      return <Colour id={id} t={t} value={String(value)} set={set} />;
    case "text":
      return <Words id={id} t={t} value={String(value)} set={set} />;
  }
}

function Toggle({ id, t, value, set }: { id: string; t: Tweak; value: boolean; set: (v: Value) => void }) {
  const [on, setOn] = useShown(value);
  return (
    <span id={id} className="tweak-switch">
      <Switch
        on={on}
        aria-label={t.label}
        onChange={(v) => {
          setOn(v);
          set(v);
        }}
      />
    </span>
  );
}

function Choose({ id, t, value, set }: { id: string; t: Tweak; value: string; set: (v: Value) => void }) {
  const options = t.options ?? [];
  if (short(t)) {
    return (
      <div id={id} className="segmented tweak-choice" role="group" aria-label={t.label}>
        {options.map((o) => (
          <Button key={o.value} type="button" aria-pressed={o.value === value} onClick={() => set(o.value)} title={o.description || undefined}>
            {o.label}
          </Button>
        ))}
      </div>
    );
  }
  return (
    <div id={id} className="tweak-select">
      <Select value={value} onChange={set} aria-label={t.label}>
        {options.map((o) => (
          <Option key={o.value} value={o.value} title={o.description || undefined}>
            {o.label}
          </Option>
        ))}
      </Select>
    </div>
  );
}

// stepOf is a slider's step where the page gives none: whole numbers
// between whole ends, else a hundredth of the way.
const stepOf = (t: Tweak, min: number, max: number) => t.step ?? (Number.isInteger(min) && Number.isInteger(max) ? 1 : (max - min) / 100);

// NumberControl is a slider, for a number with both ends, with what it's
// at beside it; else a box to type one in.
function NumberControl({ id, t, value, set }: { id: string; t: Tweak; value: number; set: (v: Value) => void }) {
  const [text, setText] = useShown(String(value));
  const put = (s: string) => {
    setText(s);
    const n = Number(s);
    if (s.trim() !== "" && takes(t, n)) set(n);
  };
  if (t.min !== undefined && t.max !== undefined) {
    return (
      <div className="tweak-range">
        <input id={id} type="range" min={t.min} max={t.max} step={stepOf(t, t.min, t.max)} value={text} onChange={(e) => put(e.target.value)} />
        <output htmlFor={id}>{said(t, Number(text))}</output>
      </div>
    );
  }
  return (
    <div className="tweak-number">
      <input id={id} type="text" inputMode="decimal" value={text} onChange={(e) => put(e.target.value)} aria-invalid={!takes(t, Number(text)) || text.trim() === ""} />
      {t.unit && <span className="dim">{t.unit}</span>}
    </div>
  );
}

// six is a colour as a colour box takes it: #rgb written out as #rrggbb.
const six = (c: string) => (c.length === 4 ? `#${c[1]}${c[1]}${c[2]}${c[2]}${c[3]}${c[3]}` : c);

function Colour({ id, t, value, set }: { id: string; t: Tweak; value: string; set: (v: Value) => void }) {
  const [shown, setShown] = useShown(six(value));
  return (
    <span className="tweak-colour">
      <span className="mono dim">{shown}</span>
      <input
        id={id}
        type="color"
        value={shown}
        aria-label={t.label}
        onChange={(e) => {
          setShown(e.target.value);
          set(e.target.value);
        }}
      />
    </span>
  );
}

function Words({ id, t, value, set }: { id: string; t: Tweak; value: string; set: (v: Value) => void }) {
  const [text, setText] = useShown(value);
  return (
    <input
      id={id}
      type="text"
      className="tweak-text"
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        if (takes(t, e.target.value)) set(e.target.value);
      }}
    />
  );
}

// TweaksToggle is the button that shows a frame's tray, or hides it:
// there for a page that declares tweaks.
export function TweaksToggle({ told, shown, onToggle }: { told: Told | undefined; shown: boolean; onToggle: () => void }) {
  if (!hasTweaks(told)) return null;
  const n = changed(told).length;
  return (
    <Button variant="ghost" size="small" aria-pressed={shown} onClick={onToggle} title="What its page says can be changed about it">
      <Icon name="sliders" size={13} /> Tweaks
      {n ? <Count>{n}</Count> : null} <Keys k="t" />
    </Button>
  );
}
