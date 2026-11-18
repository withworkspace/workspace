import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { getModels, getSettings, setSettings, type ModelInfo, type RoleRun, type Setting, type SettingChange, type SettingsInfo } from "../api";
import { defaultAccent, parseHex, pulled, showAccent, shownAs } from "../ui/accent";
import { Icon } from "../ui/Icon";
import { Callout, ErrorCard, Skeleton, toast, useTitle } from "../ui/kit";
import { Option, Select } from "../ui/Select";
import { useTheme } from "../ui/theme";
import { useLoaded } from "../load";
import { ProcessSection } from "./Process";

// The Settings page: your settings for this workspace, the ones in
// ws.local.toml. Each says what it is and where that comes from (ws's
// default, the team's ws.toml, or yours), and each role what it ends up
// running. A change is written into ws.local.toml at once, key by key, so
// the rest of the file stays as you wrote it; Reset takes yours away, and
// the team's or the default applies again.

// name is a key as it's written in the file: [section] key.
const name = (s: { section: string; key: string }) => (s.section ? `[${s.section}] ${s.key}` : s.key);

// shown is a value as the page says it: a list (picks) as its items,
// with commas.
function shown(v: unknown): string {
  if (v === undefined || v === null || v === "") return "";
  if (typeof v === "boolean") return v ? "on" : "off";
  if (Array.isArray(v)) return v.join(", ");
  return String(v);
}

// written is a change as it reads in the file, for its toast.
function written(c: SettingChange): string {
  if (c.clear) return `${name(c)} cleared`;
  const quote = (x: unknown) => (typeof x === "string" ? `"${x}"` : String(x));
  const v = Array.isArray(c.value) ? `[${c.value.map(quote).join(", ")}]` : quote(c.value);
  return `${name(c)} = ${v}`;
}

export function SettingsPage() {
  useTitle("Settings");
  const { value: info, error, set: setInfo, reload: load } = useLoaded<SettingsInfo>(getSettings, [], { keep: true });
  const [busy, setBusy] = useState(false);

  // change writes changes together; undo is what puts yours back as it was.
  const change = async (changes: SettingChange[], undo?: SettingChange[]) => {
    setBusy(true);
    try {
      setInfo(await setSettings(changes));
      toast(`${changes.map(written).join(", ")} in ws.local.toml`, {
        tone: "done",
        action: undo && { label: "Undo", run: () => void change(undo) },
      });
    } catch (e) {
      toast((e as Error).message, { tone: "danger", ms: 8000 });
      load();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="page settings-page">
      <header className="page-head">
        <h1>Settings</h1>
      </header>
      <p className="lede">
        Yours, for this workspace: kept in <code>{info ? info.file.split("/").pop() : "ws.local.toml"}</code>, which changes here are written into, key by key, the rest of
        the file as you left it. The team's are in <code>ws.toml</code>: shown here, changed there. So is how work goes here, the team's{" "}
        <a href="#process" className="link">
          process
        </a>
        , which the planner and the orchestrator read as it's written.
      </p>
      {error && <ErrorCard error={error} retry={load} />}
      {!info && !error && <Skeleton rows={8} />}
      {info && (
        <>
          {info.unknown && info.unknown.length > 0 && (
            <Callout tone="attention" title="ws doesn't read these">
              Misspelt, or in the wrong file, they're passed over without a word:
              <ul className="settings-unknown">
                {info.unknown.map((u) => (
                  <li key={u}>
                    <code>{u}</code>
                  </li>
                ))}
              </ul>
            </Callout>
          )}
          {info.groups.map((g) => (
            <section key={g.id} id={g.id} className="settings-group">
              <h2 className="group">{g.title}</h2>
              {g.about && <p className="settings-about">{g.about}</p>}
              {g.runs && <Runs runs={g.runs} />}
              <dl className="summary settings">
                {g.settings.map((s) => (
                  <SettingRow key={name(s)} s={s} group={g.settings} busy={busy} change={change} />
                ))}
              </dl>
            </section>
          ))}
        </>
      )}
      <ProcessSection />
    </div>
  );
}

// Runs is what a role ends up running, as things are set.
function Runs({ runs }: { runs: RoleRun }) {
  return (
    <div className="settings-runs">
      <div>
        Runs on{" "}
        <span className="mono">
          {runs.harness || "nothing"}
          {runs.model ? ` · ${runs.model}` : ""}
        </span>
        {!runs.model && runs.harness && <span className="dim">, on its own default model</span>}
      </div>
      {runs.problem && (
        <div className="settings-problem tone-danger">
          <Icon name="alert" /> {runs.problem}
        </div>
      )}
      {runs.note && <div className="dim">{runs.note}</div>}
    </div>
  );
}

const sources: Record<Setting["source"], { words: string; tone?: string; title: string }> = {
  default: { words: "default", title: "Neither file sets it: ws's default" },
  team: { words: "team", tone: "tone-progress", title: "From the team's ws.toml" },
  yours: { words: "yours", tone: "tone-done", title: "From your ws.local.toml" },
};

function SettingRow({
  s,
  group,
  busy,
  change,
}: {
  s: Setting;
  group: Setting[];
  busy: boolean;
  change: (changes: SettingChange[], undo?: SettingChange[]) => Promise<void>;
}) {
  const at = { section: s.section, key: s.key };
  // back is the change that puts yours back as it is now.
  const back = (x: Setting): SettingChange => (x.yours === undefined ? { section: x.section, key: x.key, clear: true } : { section: x.section, key: x.key, value: x.yours });
  const set = (value: unknown) => {
    if (value === s.value && s.source === "yours") return Promise.resolve();
    const changes: SettingChange[] = [{ ...at, value }];
    const undo = [back(s)];
    // A model belongs to the harness it was set for: changing a role's
    // harness takes your model for it away.
    const model = group.find((x) => x.kind === "model");
    if (s.kind === "harness" && model && model.yours !== undefined) {
      changes.push({ section: model.section, key: model.key, clear: true });
      undo.push(back(model));
    }
    return change(changes, undo);
  };
  // The places' two keys are one arrangement: reset together.
  const together = s.kind === "places" ? group.filter((x) => x.kind === "places") : [s];
  const reset = () => void change([{ ...at, clear: true }], together.map(back));
  const src = sources[s.source];
  return (
    <div className="summary-row setting-row">
      <dt>
        {s.label}
        <code className="setting-key">{name(s)}</code>
      </dt>
      <dd>
        <div className="setting-line">
          <Control s={s} busy={busy} set={set} />
          <span className={`chip ${src.tone ?? ""}`} title={src.title}>
            {src.words}
          </span>
          {s.source === "yours" && (
            <button className="ghost small" disabled={busy} onClick={reset} title={s.team !== undefined ? `Back to the team's: ${shown(s.team) || "none"}` : `Back to the default: ${shown(s.default) || (s.kind === "picks" ? "none" : s.kind === "color" ? "ws's own" : s.kind === "places" ? "as ws has them" : "the agent's own")}`}>
              Reset
            </button>
          )}
        </div>
        {(s.about || (s.source === "yours" && s.team !== undefined)) && (
          <div className="setting-about">
            {s.about}
            {s.source === "yours" && s.team !== undefined && <> The team's is {shown(s.team)}.</>}
          </div>
        )}
      </dd>
    </div>
  );
}

// A control's set writes a value, done when it's written (or refused).
type Set = (v: unknown) => Promise<void>;

function Control({ s, busy, set }: { s: Setting; busy: boolean; set: Set }) {
  switch (s.kind) {
    case "harness":
    case "choice":
      return (
        <Select value={shown(s.value)} disabled={busy} onChange={set} aria-label={s.label}>
          {!s.choices?.some((c) => c.value === shown(s.value)) && <Option value={shown(s.value)}>{shown(s.value) || "—"}</Option>}
          {s.choices?.map((c) => (
            <Option key={c.value} value={c.value} title={c.note}>
              {`${c.label ?? c.value}${c.note ? " (not here)" : ""}`}
            </Option>
          ))}
        </Select>
      );
    case "bool":
      return (
        <label className="setting-toggle">
          <input type="checkbox" checked={s.value === true} disabled={busy} onChange={(e) => void set(e.target.checked)} />
          {s.value ? "On" : "Off"}
        </label>
      );
    case "model":
      return <ModelInput s={s} busy={busy} set={set} />;
    case "picks":
      return <TextInput s={s} busy={busy} set={set} placeholder="claude:sonnet, cursor:gpt-5.5-high" />;
    case "places":
      // Arranged in the sidebar itself (the workspace's menu, Arrange the
      // sidebar…): here, what it says.
      return <span className="setting-places">{shown(s.value) || (s.value ? "none" : "as ws has them")}</span>;
    case "color":
      // The one colour setting is the accent, which the page wears.
      return s.section === "ui" && s.key === "accent" ? <AccentInput s={s} busy={busy} set={set} /> : <TextInput s={s} busy={busy} set={set} />;
    default:
      return <TextInput s={s} busy={busy} set={set} />;
  }
}

// TextInput commits on Enter or leaving it, if it changed.
function TextInput({ s, busy, set, list, placeholder }: { s: Setting; busy: boolean; set: Set; list?: string; placeholder?: string }) {
  const [text, setText] = useState(shown(s.value));
  useEffect(() => setText(s.source === "default" && s.kind === "model" ? "" : shown(s.value)), [s.value, s.source, s.kind]);
  const commit = () => {
    const t = text.trim();
    const was = s.source === "default" && s.kind === "model" ? "" : shown(s.value);
    if (t === was) return;
    if (t === "") {
      setText(was);
      return;
    }
    // Picks are a list, written with commas or spaces between them.
    void set(s.kind === "number" ? Number(t) : s.kind === "picks" ? t.split(/[\s,]+/).filter(Boolean) : t);
  };
  return (
    <input
      type="text"
      inputMode={s.kind === "number" ? "numeric" : undefined}
      className={s.kind === "model" || s.kind === "picks" ? "mono setting-input" : "setting-input"}
      value={text}
      list={list}
      placeholder={placeholder}
      disabled={busy}
      aria-label={s.label}
      onChange={(e) => setText(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") (e.target as HTMLInputElement).blur();
        if (e.key === "Escape") {
          setText(shown(s.value));
          (e.target as HTMLInputElement).blur();
        }
      }}
    />
  );
}

// ModelInput is free text, offering the models the role's harness lists.
function ModelInput({ s, busy, set }: { s: Setting; busy: boolean; set: Set }) {
  const id = useId();
  const harness = s.harness;
  const listed = useLoaded(harness ? () => getModels(harness) : null, [harness], { keep: true });
  const models: ModelInfo[] = listed.value ?? [];
  const note = listed.error && `Couldn't list ${harness}'s models: ${listed.error}`;
  const def = shown(s.default);
  return (
    <span className="setting-model">
      <TextInput s={s} busy={busy} set={set} list={id} placeholder={def ? `${def} (its default)` : "the agent's own default"} />
      <datalist id={id}>
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {[m.name !== m.id ? m.name : "", m.default ? "default" : ""].filter(Boolean).join(" · ")}
          </option>
        ))}
      </datalist>
      {note && <span className="setting-about">{note}</span>}
    </span>
  );
}

// AccentInput is the accent's control: a colour picker, free, and the hex
// it's at, either of which changes it. The page follows as you pick (the
// accent on <html>), and the pick's written once you let it go; one that
// isn't kept (refused, or the page left first) goes back to what's set.
// Beside it is the accent as this theme shows it: in text, as a tint, and
// on the button. A pick the theme lightens or darkens so it reads on the
// page shows as it's drawn, and says so.
function AccentInput({ s, busy, set }: { s: Setting; busy: boolean; set: Set }) {
  const saved = typeof s.value === "string" ? s.value : null;
  const [own] = useState(defaultAccent);
  const [preview, setPreview] = useState<string | null>(null);
  const picked = preview ?? saved ?? own;
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
    if (cancelled.current || t === "" || hex === (saved ?? own)) {
      cancelled.current = false;
      setPreview(null);
      return;
    }
    keep(hex ?? t, 0);
  };
  const way = pulled(picked, themed);

  // The note goes under the whole line, the source and Reset kept level
  // with the picker.
  return (
    <>
      <span className="setting-accent">
        <input ref={picker} type="color" value={picked} disabled={busy} aria-label={s.label} onChange={(e) => setPreview(e.target.value)} />
        <input
          type="text"
          className="mono setting-input"
          value={editing ?? picked}
          disabled={busy}
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
        <span className="accent-shown" title={`As the ${dark ? "dark" : "light"} theme shows it`}>
          <span className="accent-shown-text">Text</span>
          <span className="accent-shown-tint">Selected</span>
          <span className="accent-shown-button">Button</span>
        </span>
      </span>
      {way && (
        <span className="setting-about accent-note">
          The {dark ? "dark" : "light"} theme shows it {way}, as <code>{themed}</code>, so it reads on the page.
        </span>
      )}
    </>
  );
}
