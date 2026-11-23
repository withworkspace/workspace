// How things look, written down, to compare before a change with after.
//
// A change to how a kind of thing is styled (every text field, every
// button) is meant to move nothing, or only what it says it moves. This
// goes through a try's web UI, scene by scene, and writes down what the
// browser made of each of those things: its size, and the styles that
// draw it, at rest and with the focus. Taken before the change and after,
// the two are compared, and what differs is said.
//
//   just try up marketing            a try to look at (more seeds, more to see)
//   cd web
//   node scripts/looks.mjs snap before.json        before the change
//   just try restart                               (the change made)
//   node scripts/looks.mjs snap after.json
//   node scripts/looks.mjs diff before.json after.json
//
// snap takes:
//   --of <selector>    what's looked at (default: text fields)
//   --states a,b       rest, focus, hover (default: rest,focus)
//   --width <px>       the window's width (default 1280; 390 for a phone's)
//   --dark             in the dark theme
//   --only <scene>     only the scenes whose name has this in it (repeat for more)
//   --list             say what each scene found, not only how many
//
// The try is this checkout's (scripts/try.sh info link), or WS_LINK's.
// The scenes are in looks.scenes.mjs: a page, and what's pressed on it to
// bring more into view. They change nothing: a dialog is opened, not
// sent. A scene that finds nothing is said, so a seed that's missing, or
// a button that's been renamed, shows.
//
// Things are paired by scene and by where they come in it, so the two
// snaps have to be of the same try, seeded the same.
import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { chromium } from "playwright";
import { scenes } from "./looks.scenes.mjs";

const here = dirname(new URL(import.meta.url).pathname);

// Text fields: what's typed in. Not a box that's ticked, a colour or a
// slider, nor the hidden inputs a part keeps for a form.
const fields = 'textarea, input:not([type]), input[type="text"], input[type="search"], input[type="url"], input[type="email"], input[type="number"]';

// What draws a thing. Its box is measured too.
const props = [
  "display",
  "position",
  "box-sizing",
  "padding-top",
  "padding-right",
  "padding-bottom",
  "padding-left",
  "margin-top",
  "margin-right",
  "margin-bottom",
  "margin-left",
  "border-top-width",
  "border-right-width",
  "border-bottom-width",
  "border-left-width",
  "border-top-style",
  "border-top-color",
  "border-right-color",
  "border-bottom-color",
  "border-left-color",
  "border-top-left-radius",
  "border-top-right-radius",
  "border-bottom-right-radius",
  "border-bottom-left-radius",
  "background-color",
  "background-image",
  "color",
  "font-family",
  "font-size",
  "font-weight",
  "font-style",
  "line-height",
  "letter-spacing",
  "text-align",
  "text-transform",
  "white-space",
  "box-shadow",
  "outline-style",
  "outline-width",
  "outline-color",
  "outline-offset",
  "opacity",
  "min-width",
  "max-width",
  "min-height",
  "max-height",
  "flex-grow",
  "flex-shrink",
  "flex-basis",
  "align-self",
  "resize",
  "overflow-x",
  "overflow-y",
  "cursor",
  "transition-property",
  "transition-duration",
  "transform",
  "gap",
  "justify-content",
  "align-items",
];

function args(argv) {
  const out = { _: [], only: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--dark" || a === "--list") out[a.slice(2)] = true;
    else if (a === "--only") out.only.push(argv[++i]);
    else if (a.startsWith("--")) out[a.slice(2)] = argv[++i];
    else out._.push(a);
  }
  return out;
}

function tryLink() {
  if (process.env.WS_LINK) return process.env.WS_LINK;
  try {
    return execFileSync(join(here, "../../scripts/try.sh"), ["info", "link"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
  } catch {
    throw new Error("no try is up for this checkout (just try up marketing), and WS_LINK isn't set");
  }
}

// A step of a scene: a key pressed ({ key }), something clicked
// ({ click: a Playwright selector }), words typed ({ type }), or a wait
// ({ wait: ms }). then says how long to give the page after it.
async function step(page, s) {
  if (s.key) await page.keyboard.press(s.key);
  else if (s.type) await page.keyboard.type(s.type);
  else if (s.wait) await page.waitForTimeout(s.wait);
  else if (s.click) {
    // The first that's shown: a page may keep another for a phone's width.
    const one = page.locator(s.click).locator("visible=true").first();
    await one.waitFor({ state: "visible", timeout: 3000 });
    await one.click();
  }
  await page.waitForTimeout(s.then ?? 350);
}

// What's in view of what's looked at, each marked so it can be found again.
const mark = (of) =>
  `(() => {
    const seen = [];
    let n = 0;
    for (const el of document.querySelectorAll(${JSON.stringify(of)})) {
      const r = el.getBoundingClientRect();
      const cs = getComputedStyle(el);
      if (r.width < 2 || r.height < 2 || cs.visibility === "hidden" || el.closest("[aria-hidden='true'], [inert]")) continue;
      el.setAttribute("data-looks", String(n));
      const classes = [...el.classList].map((c) => c.replace(/_[A-Za-z0-9-]{5}$/, "")).join(" ");
      seen.push({ n, what: el.tagName.toLowerCase() + (el.type && el.tagName === "INPUT" ? "[" + el.type + "]" : ""), says: (el.getAttribute("aria-label") || el.getAttribute("placeholder") || el.getAttribute("title") || el.id || (el.textContent || "").trim().slice(0, 40) || "").slice(0, 70), classes });
      n++;
    }
    return seen;
  })()`;

const read = (n, names) =>
  `(() => {
    const el = document.querySelector('[data-looks="${n}"]');
    if (!el) return null;
    const cs = getComputedStyle(el);
    const r = el.getBoundingClientRect();
    const out = { width: String(Math.round(r.width * 100) / 100), height: String(Math.round(r.height * 100) / 100) };
    for (const p of ${JSON.stringify(names)}) out[p] = cs.getPropertyValue(p);
    return out;
  })()`;

async function snap(o) {
  const outFile = o._[1];
  if (!outFile) throw new Error("snap <out.json>");
  const of = o.of ?? fields;
  const states = (o.states ?? "rest,focus").split(",");
  const link = tryLink();
  const origin = new URL(link).origin;
  const browser = await chromium.launch();
  const context = await browser.newContext({ viewport: { width: Number(o.width ?? 1280), height: 900 }, colorScheme: o.dark ? "dark" : "light" });
  const page = await context.newPage();
  page.setDefaultTimeout(8000);
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e).slice(0, 200)));
  await page.goto(link);
  await page.waitForTimeout(1200);
  const cdp = await context.newCDPSession(page);
  await cdp.send("DOM.enable");
  await cdp.send("CSS.enable");
  const result = { of, states, width: Number(o.width ?? 1280), dark: !!o.dark, scenes: {} };
  const wanted = scenes.filter((s) => !o.only.length || o.only.some((x) => s.name.includes(x)));
  for (const scene of wanted) {
    let failed;
    try {
      await page.goto(origin + scene.path);
      await page.waitForTimeout(scene.settle ?? 1500);
      // Nothing moves while it's read: a style in the middle of a
      // transition is neither the one before nor the one after.
      await page.addStyleTag({ content: "*, *::before, *::after { transition: none !important; animation: none !important; caret-color: transparent !important; }" });
      for (const s of scene.steps ?? []) await step(page, s);
    } catch (e) {
      failed = String(e).split("\n")[0].slice(0, 160);
    }
    const seen = await page.evaluate(mark(of));
    const things = [];
    for (const t of seen) {
      const looks = {};
      for (const state of states) {
        if (state === "focus") {
          await page.evaluate(`document.querySelector('[data-looks="${t.n}"]')?.focus({ preventScroll: true })`);
          await page.waitForTimeout(60);
        }
        let node;
        if (state === "hover") {
          const { root } = await cdp.send("DOM.getDocument", { depth: 0 });
          node = (await cdp.send("DOM.querySelector", { nodeId: root.nodeId, selector: `[data-looks="${t.n}"]` })).nodeId;
          if (node) await cdp.send("CSS.forcePseudoState", { nodeId: node, forcedPseudoClasses: ["hover"] });
        }
        looks[state] = await page.evaluate(read(t.n, props));
        if (state === "focus") await page.evaluate(`document.querySelector('[data-looks="${t.n}"]')?.blur()`);
        if (state === "hover" && node) await cdp.send("CSS.forcePseudoState", { nodeId: node, forcedPseudoClasses: [] });
      }
      things.push({ what: t.what, says: t.says, classes: t.classes, looks });
    }
    result.scenes[scene.name] = { path: scene.path, failed, things };
    const note = failed ? `  (a step failed: ${failed})` : "";
    console.log(`${String(things.length).padStart(3)}  ${scene.name}${note}`);
    if (o.list) for (const t of things) console.log(`       ${t.what} "${t.says}" ${t.classes ? "." + t.classes.replace(/ /g, ".") : ""}`);
  }
  await browser.close();
  writeFileSync(outFile, JSON.stringify(result, null, 1));
  const total = Object.values(result.scenes).reduce((n, s) => n + s.things.length, 0);
  const empty = Object.entries(result.scenes).filter(([, s]) => !s.things.length).map(([n]) => n);
  console.log(`\n${total} looked at, in ${wanted.length} scenes, written to ${outFile}`);
  if (empty.length) console.log(`nothing found in: ${empty.join(", ")}`);
  if (errors.length) console.log(`the page threw: ${[...new Set(errors)].slice(0, 5).join(" | ")}`);
}

function diff(o) {
  const [, beforeFile, afterFile] = o._;
  if (!beforeFile || !afterFile) throw new Error("diff <before.json> <after.json>");
  const before = JSON.parse(readFileSync(beforeFile, "utf8"));
  const after = JSON.parse(readFileSync(afterFile, "utf8"));
  let differing = 0;
  let compared = 0;
  const tally = new Map();
  for (const [name, b] of Object.entries(before.scenes)) {
    const a = after.scenes[name];
    if (!a) {
      console.log(`${name}: not in ${afterFile}`);
      continue;
    }
    const lines = [];
    if (a.things.length !== b.things.length) lines.push(`  ${b.things.length} before, ${a.things.length} after: paired as far as they go`);
    for (let i = 0; i < Math.min(a.things.length, b.things.length); i++) {
      const [tb, ta] = [b.things[i], a.things[i]];
      compared++;
      const moved = [];
      if (tb.what !== ta.what || tb.says !== ta.says) moved.push(`    it's another thing: ${tb.what} "${tb.says}" before, ${ta.what} "${ta.says}" after`);
      for (const state of Object.keys(tb.looks)) {
        const [lb, la] = [tb.looks[state], ta.looks[state]];
        if (!lb || !la) continue;
        for (const p of Object.keys(lb)) {
          if (lb[p] === la[p]) continue;
          moved.push(`    ${state.padEnd(5)} ${p}: ${lb[p]} → ${la[p]}`);
          const k = `${state} ${p}: ${lb[p]} → ${la[p]}`;
          tally.set(k, (tally.get(k) ?? 0) + 1);
        }
      }
      if (moved.length) {
        differing++;
        lines.push(`  ${ta.what} "${ta.says}"${ta.classes ? ` .${ta.classes.replace(/ /g, ".")}` : ""}`, ...moved);
      }
    }
    if (lines.length) console.log(`${name}\n${lines.join("\n")}`);
  }
  for (const name of Object.keys(after.scenes)) if (!before.scenes[name]) console.log(`${name}: not in ${beforeFile}`);
  console.log(`\n${compared} compared, ${differing} differ`);
  if (tally.size) {
    console.log("\nWhat moved, and in how many:");
    for (const [k, n] of [...tally.entries()].sort((x, y) => y[1] - x[1]).slice(0, 40)) console.log(`${String(n).padStart(4)}  ${k}`);
  }
  process.exitCode = differing ? 1 : 0;
}

const o = args(process.argv.slice(2));
const run = { snap, diff, scenes: () => scenes.forEach((s) => console.log(`${s.name}  ${s.path}`)) }[o._[0]];
if (!run) {
  console.error("looks.mjs snap <out.json> | diff <before.json> <after.json> | scenes   (the top of the file says the rest)");
  process.exit(2);
}
await run(o);
