// ws.tweaks: what an artifact's page says can be changed about it, for
// ws to draw the controls. ws writes this into every frame it serves,
// ahead of the page's own scripts.
(() => {
  "use strict";
  const MOST = { tweaks: 40, presets: 20, options: 30, text: 200, label: 80, description: 300 };
  const NAME = /^[A-Za-z][A-Za-z0-9_-]{0,39}$/;
  const COLOR = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
  const UNIT = /^[A-Za-z%]{0,8}$/;
  const KINDS = ["toggle", "number", "text", "color"];
  // The page that frames this one: ws's. Opened alone there's none, and
  // the state is what the address says.
  const host = window.parent !== window ? window.parent : null;
  const root = document.documentElement;

  let declared = false;
  let defs = [];
  let presets = [];
  let opening = {};
  let state = {};
  let problems = [];
  const listeners = new Set();

  const isObject = (v) => v !== null && typeof v === "object" && !Array.isArray(v);
  const words = (v, most) => (typeof v === "string" ? v.trim().slice(0, most) : "");
  const number = (v) => (typeof v === "number" && Number.isFinite(v) ? v : undefined);

  // read is a value as a tweak takes it, or undefined for one it doesn't:
  // typed, as a preset or a control gives it, or as the address writes it
  // (on and off, a number's digits).
  function read(def, v) {
    switch (def.kind) {
      case "choice":
        return typeof v === "string" && def.options.some((o) => o.value === v) ? v : undefined;
      case "toggle":
        return v === true || v === "on" ? true : v === false || v === "off" ? false : undefined;
      case "number": {
        const n = typeof v === "number" ? v : typeof v === "string" && v.trim() !== "" ? Number(v) : NaN;
        return Number.isFinite(n) && (def.min === undefined || n >= def.min) && (def.max === undefined || n <= def.max) ? n : undefined;
      }
      case "color":
        return typeof v === "string" && COLOR.test(v) ? v.toLowerCase() : undefined;
      case "text":
        return typeof v === "string" && v.length <= MOST.text ? v : undefined;
    }
    return undefined;
  }

  // nothing is a tweak's value where the first preset gives it none.
  function nothing(def) {
    switch (def.kind) {
      case "choice":
        return def.options[0].value;
      case "toggle":
        return false;
      case "number":
        return def.min ?? 0;
      case "color":
        return "#000000";
    }
    return "";
  }

  // tweakOf reads one tweak as it's declared: its kind from its options,
  // from what it says (type, min, max), or from the value the first preset
  // gives it.
  function tweakOf(name, said, first) {
    if (!NAME.test(name)) {
      problems.push(`${name}: a tweak's name is letters, digits, - and _, starting with a letter`);
      return undefined;
    }
    const d = isObject(said) ? said : {};
    const def = { name, label: words(d.label, MOST.label) || name, description: words(d.description, MOST.description), kind: "" };
    if (Array.isArray(d.options)) {
      def.kind = "choice";
      def.options = [];
      for (const o of d.options.slice(0, MOST.options)) {
        const value = typeof o === "string" ? o : isObject(o) ? o.value : undefined;
        if (typeof value !== "string" || !value || value.length > MOST.text) {
          problems.push(`${name}: an option is a string, or { value, label }`);
        } else if (def.options.some((x) => x.value === value)) {
          problems.push(`${name}: ${value} is there twice`);
        } else {
          def.options.push({ value, label: (isObject(o) && words(o.label, MOST.label)) || value, description: isObject(o) ? words(o.description, MOST.description) : "" });
        }
      }
      if (!def.options.length) {
        problems.push(`${name}: it has no options`);
        return undefined;
      }
      return def;
    }
    const v = first[name];
    if (KINDS.includes(d.type)) def.kind = d.type;
    else if (number(d.min) !== undefined || number(d.max) !== undefined) def.kind = "number";
    else if (typeof v === "boolean") def.kind = "toggle";
    else if (typeof v === "number") def.kind = "number";
    else if (typeof v === "string") def.kind = "text";
    if (!def.kind) {
      problems.push(`${name}: the first preset gives it no value, so there's no telling what it is`);
      return undefined;
    }
    if (def.kind === "number") {
      const min = number(d.min);
      const max = number(d.max);
      if (min !== undefined && max !== undefined && min > max) problems.push(`${name}: its min is over its max`);
      else {
        if (min !== undefined) def.min = min;
        if (max !== undefined) def.max = max;
      }
      const step = number(d.step);
      if (step !== undefined && step > 0) def.step = step;
      if (typeof d.unit === "string" && UNIT.test(d.unit)) def.unit = d.unit;
    }
    return def;
  }

  // declare reads what the page says: its tweaks, in order, and its
  // presets. The first preset is what the page opens as, so it's made to
  // say every tweak.
  function declare(said) {
    const d = isObject(said) ? said : {};
    const list = Array.isArray(d.presets) ? d.presets : [];
    const first = isObject(list[0]) && isObject(list[0].state) ? list[0].state : {};
    problems = [];
    const before = defs;
    defs = [];
    for (const [name, t] of Object.entries(isObject(d.tweaks) ? d.tweaks : {})) {
      if (defs.length >= MOST.tweaks) {
        problems.push(`there are more than ${MOST.tweaks} tweaks: the rest are left out`);
        break;
      }
      const def = tweakOf(name, t, first);
      if (def) defs.push(def);
    }
    presets = [];
    list.slice(0, MOST.presets).forEach((p, i) => {
      const label = (isObject(p) && words(p.label, MOST.label)) || (i ? `Preset ${i + 1}` : "As it opens");
      const preset = { label, description: isObject(p) ? words(p.description, MOST.description) : "", state: {} };
      for (const [name, v] of Object.entries(isObject(p) && isObject(p.state) ? p.state : {})) {
        const def = defs.find((x) => x.name === name);
        const value = def && read(def, v);
        if (!def) problems.push(`${label}: there's no tweak named ${name}`);
        else if (value === undefined) problems.push(`${label}: ${name} can't be ${JSON.stringify(v)}`);
        else preset.state[name] = value;
      }
      presets.push(preset);
    });
    if (defs.length && !presets.length) {
      problems.push("there are no presets: the first is what the page opens as");
      presets.push({ label: "As it opens", description: "", state: {} });
    }
    opening = {};
    for (const def of defs) {
      if (!(def.name in presets[0].state)) {
        if (list.length) problems.push(`${presets[0].label}: it gives ${def.name} no value`);
        presets[0].state[def.name] = nothing(def);
      }
      opening[def.name] = presets[0].state[def.name];
    }
    // The state is the opening one, with what the address says of it.
    state = { ...opening };
    for (const [key, raw] of new URLSearchParams(location.search)) {
      const def = key.startsWith("t.") ? defs.find((x) => x.name === key.slice(2)) : undefined;
      const value = def ? read(def, raw) : undefined;
      if (def && value !== undefined) state[def.name] = value;
    }
    for (const def of before) if (!defs.some((x) => x.name === def.name)) show(def, undefined);
    for (const def of defs) show(def, state[def.name]);
    declared = true;
  }

  // show writes a tweak's value where the page's styles read it: data-name
  // on <html> (there or not, for a toggle), and --name for a number, with
  // its unit, and a colour. undefined takes them away.
  function show(def, value) {
    const attr = `data-${def.name}`;
    if (value === undefined || value === false) root.removeAttribute(attr);
    else root.setAttribute(attr, value === true ? "" : String(value));
    if (def.kind !== "number" && def.kind !== "color") return;
    if (value === undefined) root.style.removeProperty(`--${def.name}`);
    else root.style.setProperty(`--${def.name}`, def.kind === "number" ? `${value}${def.unit ?? ""}` : value);
  }

  function changed() {
    const now = { ...state };
    for (const fn of listeners) fn(now);
    window.dispatchEvent(new CustomEvent("ws:tweaks", { detail: now }));
  }

  // set changes some of the tweaks, to values they take: the rest of what
  // it's given is dropped.
  function set(some) {
    if (!declared || !isObject(some)) return;
    let any = false;
    for (const def of defs) {
      const value = def.name in some ? read(def, some[def.name]) : undefined;
      if (value === undefined || value === state[def.name]) continue;
      state[def.name] = value;
      show(def, value);
      any = true;
    }
    if (!any) return;
    changed();
    if (host) host.postMessage({ ws: "tweaks:state", state: { ...state } }, "*");
  }

  // tell says what's declared to ws, which draws the controls from it.
  function tell() {
    if (host && declared) host.postMessage({ ws: "tweaks", tweaks: defs, presets, opening, state: { ...state }, problems }, "*");
  }

  window.addEventListener("message", (e) => {
    if (!host || e.source !== host || !isObject(e.data)) return;
    if (e.data.ws === "tweaks:hello") tell();
    if (e.data.ws === "tweaks:set") set(e.data.state);
  });

  function tweaks(said, onChange) {
    declare(said);
    listeners.clear();
    if (typeof onChange === "function") listeners.add(onChange);
    tell();
    changed();
    return { ...state };
  }
  tweaks.set = set;
  Object.defineProperty(tweaks, "state", { get: () => ({ ...state }) });

  window.ws = Object.assign(window.ws || {}, { tweaks });
})();
