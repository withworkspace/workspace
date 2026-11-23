import type { V1 } from "@withworkspace/ui-lib/protocol";
import { type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import * as api from "../api";
import { Icon } from "../ui/Icon";
import type { Anchor } from "../ui/hover";
import type { UiProps } from "./Uis";
import { Button } from "../ui/Button";
import { chipClass } from "../ui/Chip";
import { Radio, RadioGroup } from "../ui/RadioGroup";

// ws's UIs for a plugin's thing that lives in a repo, asked for by the
// plugin's page from a button of its own (ui.ts): putting one there
// (thing-promote) and saving it (thing-save). Where a thing goes, the
// commit and the landing are ws's, so what says them is ws's too, the
// same for every plugin. ws asks the plugin for the thing itself, and
// tells it where it is: the page is given what was done, and nothing
// passes through it.

// said is why something failed, in its own words.
const said = (err: unknown) => (err instanceof Error ? err.message : String(err));

// asked is the thing a page asked about, as its UI was given it.
function asked(of: Record<string, unknown>): V1.ThingAsked {
  return { kind: String(of.kind ?? ""), ref: String(of.ref ?? ""), ...(typeof of.title === "string" && of.title ? { title: of.title } : {}) };
}

// OverFrame is a panel of ws's by something in a plugin's page: under the
// button it was asked from, its right edge on the button's where there's
// no room to its right, kept in the window. Escape, or a press anywhere
// else, dismisses it, unless it's busy.
function OverFrame({ anchor, label, busy, dismiss, children }: { anchor?: Anchor; label: string; busy: boolean; dismiss: () => void; children: ReactNode }) {
  const panel = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number }>();
  useLayoutEffect(() => {
    const place = () => {
      if (!panel.current) return;
      const p = panel.current.getBoundingClientRect();
      if (!anchor?.isConnected) {
        setPos({ top: 72, left: Math.max(12, (window.innerWidth - p.width) / 2) });
        return;
      }
      const at = anchor.getBoundingClientRect();
      const left = at.left + p.width <= window.innerWidth - 12 ? at.left : at.right - p.width;
      const below = at.bottom + 6;
      setPos({
        top: below + p.height > window.innerHeight - 12 ? Math.max(12, at.top - 6 - p.height) : below,
        left: Math.max(12, Math.min(left, window.innerWidth - p.width - 12)),
      });
    };
    place();
    const grown = new ResizeObserver(place);
    if (panel.current) grown.observe(panel.current);
    window.addEventListener("resize", place);
    return () => {
      grown.disconnect();
      window.removeEventListener("resize", place);
    };
  }, [anchor]);
  // What's said through a ref, so the listeners are set once.
  const leave = useRef(dismiss);
  leave.current = busy ? () => {} : dismiss;
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.stopPropagation();
      leave.current();
    };
    const press = (e: PointerEvent) => {
      if (e.target instanceof Node && panel.current?.contains(e.target)) return;
      leave.current();
    };
    document.addEventListener("keydown", key, true);
    document.addEventListener("pointerdown", press, true);
    return () => {
      document.removeEventListener("keydown", key, true);
      document.removeEventListener("pointerdown", press, true);
    };
  }, []);
  return createPortal(
    <>
      {/* The frame takes no presses while it's up: one on it dismisses, as one anywhere does. */}
      <div className="over-frame-catch" onPointerDown={() => leave.current()} />
      <div ref={panel} className="over-frame" role="dialog" aria-label={label} style={pos ? { top: pos.top, left: pos.left } : { top: -9999, left: -9999 }}>
        {children}
      </div>
    </>,
    document.body,
  );
}

// grouped is where a thing would live in a place, in a folder of it:
// records:sketches/release-flow/ in cross-stack is
// records:sketches/cross-stack/release-flow/.
function grouped(where: string, under: string): string {
  const group = under
    .split("/")
    .map((s) => s.trim())
    .filter(Boolean)
    .join("/");
  const at = where.replace(/\/$/, "").lastIndexOf("/");
  const cut = at < 0 ? where.indexOf(":") : at;
  return group && cut >= 0 ? `${where.slice(0, cut + 1)}${group}/${where.slice(cut + 1)}` : where;
}

// ThingPromoteUi puts one of a plugin's own things in a repo: it says
// where it would live, or, where its kind's things go in more places than
// one, has you pick among them; takes a folder of the place to group it
// in, where its kind's things may be grouped, with those in use to pick
// from; takes somewhere else if you say; and does it when you say. What
// comes of it is where the thing is now.
export function ThingPromoteUi({ asked: asking, plugin, anchor, done }: UiProps) {
  const thing = asked(asking.with);
  const [places, setPlaces] = useState<api.ThingPlace[]>();
  const [failed, setFailed] = useState("");
  // Which of the places, or somewhere else, said in words.
  const [pick, setPick] = useState<number | "else">(0);
  const [elsewhere, setElsewhere] = useState("");
  // A folder of the place picked, to group it in: for a place that takes one.
  const [under, setUnder] = useState("");
  const [busy, setBusy] = useState(false);
  const go = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    let live = true;
    api.thingOffered(plugin, thing.kind, thing.title ?? "").then(
      (o) => {
        if (!live) return;
        setPlaces(o.places);
        // Nowhere to offer: it's said in words.
        if (o.places.length === 0) setPick("else");
      },
      (err) => {
        if (!live) return;
        setFailed(said(err));
        setPlaces([]);
        setPick("else");
      },
    );
    return () => {
      live = false;
    };
  }, [plugin, thing.kind, thing.title]);
  useEffect(() => go.current?.focus(), [places]);

  // Another place has folders of its own: the one said for the last isn't carried to it.
  const choose = (next: number | "else") => {
    if (next !== pick) setUnder("");
    setPick(next);
  };
  const place = pick === "else" ? undefined : places?.[pick];
  const group = place?.subfolders ? under.trim().replace(/^\/+|\/+$/g, "") : "";
  const to = pick === "else" ? elsewhere.trim() : place?.where;
  const promote = async () => {
    if (!to) return;
    setBusy(true);
    setFailed("");
    try {
      // A place is named, not its folder: ws names the thing's own afresh, as the repo is on its remote then. The first is where one goes with nothing said.
      const where = pick === "else" ? { to } : { ...(pick === 0 ? {} : { place: place?.place }), ...(group ? { under: group } : {}) };
      done(await api.promoteThing(plugin, { kind: thing.kind, ref: thing.ref, ...where }));
    } catch (err) {
      setFailed(said(err));
      setBusy(false);
    }
  };
  const what = thing.title || thing.ref;
  const several = (places?.length ?? 0) > 1;
  return (
    <OverFrame anchor={anchor} label={`Put ${what} in a repo`} busy={busy} dismiss={() => done()}>
      <form
        className="thing-ui"
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void promote();
        }}
      >
        <strong className="thing-ui-title">Put {what} in a repo</strong>
        <p>Everyone with the repo has it then. What you change after is on this machine until you save it there.</p>
        {!places && <p className="thing-ui-where">Working out where it would live…</p>}
        {places && !several && pick !== "else" && place && (
          <p className="thing-ui-where">
            It'll live at <code>{grouped(place.where, group)}</code>
            <Button variant="link" type="button" className="thing-ui-else" disabled={busy} onClick={() => setPick("else")}>
              Somewhere else…
            </Button>
          </p>
        )}
        {places && several && (
          <fieldset className="thing-ui-places" disabled={busy}>
            <legend>Where it'll live</legend>
            <RadioGroup className="thing-ui-place-list" value={String(pick)} onChange={(v) => choose(v === "else" ? "else" : Number(v))} disabled={busy} aria-label="Where it'll live">
              {places.map((p, i) => (
                <Radio key={p.where} value={String(i)}>
                  <span className="thing-ui-place">
                    <span className="thing-ui-repo">{p.label}</span>
                    <code>{pick === i ? grouped(p.where, group) : p.where}</code>
                  </span>
                </Radio>
              ))}
              <Radio value="else">Somewhere else…</Radio>
            </RadioGroup>
          </fieldset>
        )}
        {place?.subfolders && (
          <div className="thing-ui-group">
            <label>
              <span className="dim">In a folder there, if you like</span>
              <input type="text" className="mono" value={under} placeholder="cross-stack" aria-label={`A folder of ${place.label} to group it in`} list="thing-ui-groups" disabled={busy} onChange={(e) => setUnder(e.currentTarget.value)} />
            </label>
            <datalist id="thing-ui-groups">
              {(place.groups ?? []).map((g) => (
                <option key={g} value={g} />
              ))}
            </datalist>
            {!!place.groups?.length && (
              <div className="thing-ui-groups" role="group" aria-label="The folders in use there">
                {place.groups.slice(0, 8).map((g) => (
                  <button key={g} type="button" className={chipClass(undefined, "thing-ui-group")} aria-pressed={group === g} disabled={busy} onClick={() => setUnder(group === g ? "" : g)}>
                    {g}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
        {places && pick === "else" && (
          <label className="thing-ui-where">
            {!several && <span className="dim">It'll live at</span>}
            {/* biome-ignore lint/a11y/noAutofocus: the field is what was asked for */}
            <input type="text" className="mono" value={elsewhere} placeholder="repo:folder/" aria-label="Where it'll live, as repo:folder/" autoFocus disabled={busy} onChange={(e) => setElsewhere(e.currentTarget.value)} />
          </label>
        )}
        {place && !place.direct && <p className="dim">{place.label} takes pull requests: it'll be left in a capsule, for you to open one from.</p>}
        {failed && (
          <p className="thing-ui-failed" role="alert">
            {failed}
          </p>
        )}
        <div className="thing-ui-buttons">
          <Button variant="primary" size="small" type="submit" ref={go} disabled={busy || !to}>
            {busy ? "Promoting…" : "Promote"}
          </Button>
          <Button variant="ghost" size="small" type="button" disabled={busy} onClick={() => done()}>
            Cancel
          </Button>
        </div>
      </form>
    </OverFrame>
  );
}

// How long what came of a save stays by the button before it goes.
const shown = 1400;

// ThingSaveUi saves one of a plugin's own things to the repo it lives in:
// asking for it is pressing save, so it saves at once, says by the button
// how it's going and what came of it, and goes. It stays only when it
// couldn't, with why.
export function ThingSaveUi({ asked: asking, plugin, anchor, done }: UiProps) {
  const thing = asked(asking.with);
  const [state, setState] = useState<{ at: "saving" } | { at: "saved"; did: api.ThingSaved } | { at: "failed"; why: string }>({ at: "saving" });
  const [again, setAgain] = useState(0);
  // Asked once for each try: a render made twice (StrictMode) saves once.
  const tried = useRef(-1);
  const finish = useRef(done);
  finish.current = done;
  // biome-ignore lint/correctness/useExhaustiveDependencies: again is the asking
  useEffect(() => {
    if (tried.current === again) return;
    tried.current = again;
    setState({ at: "saving" });
    let timer: ReturnType<typeof setTimeout> | undefined;
    api.saveThing(plugin, { kind: thing.kind, ref: thing.ref }).then(
      (did) => {
        setState({ at: "saved", did });
        timer = setTimeout(() => finish.current(did), shown);
      },
      (err) => setState({ at: "failed", why: said(err) }),
    );
    return () => clearTimeout(timer);
  }, [plugin, thing.kind, thing.ref, again]);

  const what = thing.title || thing.ref;
  const did = state.at === "saved" ? state.did : undefined;
  return (
    <OverFrame anchor={anchor} label={`Saving ${what}`} busy={state.at === "saving"} dismiss={() => done(did)}>
      <div className="thing-ui" role="status">
        {state.at === "saving" && (
          <p className="thing-ui-line">
            <Icon name="spinner" size={14} className="spin" /> Saving {what} to its repo…
          </p>
        )}
        {did && (
          <>
            <p className="thing-ui-line">
              <Icon name="check" size={14} /> {did.capsule ? "Saved in a capsule" : did.changed ? "Saved" : "Nothing to save"}
              <code>
                {did.thing.repo}:{did.thing.path}
              </code>
            </p>
            {did.merged && <p className="dim">It had changed there since: what was there is in it now.</p>}
            {did.capsule && <p className="dim">{did.thing.repo} takes pull requests: it's in {did.capsule}, for you to open one from.</p>}
            {!did.changed && !did.capsule && <p className="dim">It's there as it is here.</p>}
          </>
        )}
        {state.at === "failed" && (
          <>
            <strong className="thing-ui-title">{what} wasn't saved</strong>
            <p className="thing-ui-failed" role="alert">
              {state.why}
            </p>
            <div className="thing-ui-buttons">
              <Button variant="primary" size="small" type="button" onClick={() => setAgain((n) => n + 1)}>
                Try again
              </Button>
              <Button variant="ghost" size="small" type="button" onClick={() => done()}>
                Close
              </Button>
            </div>
          </>
        )}
      </div>
    </OverFrame>
  );
}
