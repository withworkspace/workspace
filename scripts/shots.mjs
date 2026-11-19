#!/usr/bin/env node
// The README's pictures, taken again: the web UI of a try seeded with
// scripts/seeds/marketing.sh, in a headless Chrome, light and dark.
//
//   node scripts/shots.mjs [name...]      (just shots)
//
// With no names it takes the shots the README shows, into
// docs/assets/shots/ as <name>-light.webp and <name>-dark.webp; with
// names, those of the shots below (all, for every one). It makes its own
// try (scripts/try.sh up marketing) and tears it down after.
//
//   --try <name>   take them of a try that's already up (it's left up)
//   --out <dir>    where they go (default docs/assets/shots)
//   --keep         leave the try it made up, and say where it is
//
// It needs node 22 or later (for its WebSocket) and a Chrome or Chromium:
// $CHROME names one that isn't where it looks.
//
// A shot is a page at a size, and may say what to bring into view first.
// Add one here when a picture is worth taking more than once.
import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const shots = [
  // The README's.
  { name: "mission", path: "/missions/invoice-totals-off-by-a-penny", readme: true },
  { name: "missions", path: "/", readme: true },
  {
    name: "review",
    path: "/missions/invoice-totals-off-by-a-penny/review/api/tax-rounding?view=changes",
    // Its comment, in the middle of the page.
    show: "Half away from zero",
    readme: true,
  },
  { name: "verify", path: "/missions/export-invoices-as-csv?tab=verify&b=b1", readme: true },
  // The rest, for when they're asked for by name.
  { name: "reviews", path: "/reviews" },
  { name: "documents", path: "/documents" },
];

// The page's size, and how many pixels a point of it is drawn with: a
// README's column is about 1000 points wide, so this is sharp there.
const size = { width: 1280, height: 800, scale: 2 };

const here = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const flag = (name) => {
  const i = args.indexOf(name);
  if (i < 0) return undefined;
  const [, value] = args.splice(i, 2);
  if (value === undefined) fail(`${name} takes a value`);
  return value;
};

// What went wrong, in words: said at the end, once what was started has
// been stopped.
function fail(why) {
  throw new Error(why);
}

function tryCommand(...a) {
  const r = spawnSync(join(here, "scripts/try.sh"), a, { encoding: "utf8", stdio: ["ignore", "pipe", "inherit"] });
  if (r.status !== 0) fail(`scripts/try.sh ${a.join(" ")} failed`);
  return r.stdout.trim();
}

function chromePath() {
  const places = [
    process.env.CHROME,
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
    ...["google-chrome", "google-chrome-stable", "chromium", "chromium-browser"].map((n) => spawnSync("which", [n], { encoding: "utf8" }).stdout.trim()),
  ];
  const found = places.find((p) => p && existsSync(p));
  if (!found) fail("no Chrome or Chromium found: name one with $CHROME");
  return found;
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// A Chrome with a profile of its own, on a port it picks (it writes
// which in the profile), so it's nobody else's browser that's driven.
async function startChrome() {
  const profile = mkdtempSync(join(tmpdir(), "ws-shots-"));
  const proc = spawn(chromePath(), ["--headless=new", "--remote-debugging-port=0", `--user-data-dir=${profile}`, "--no-first-run", "--hide-scrollbars", "about:blank"], { stdio: "ignore" });
  const portFile = join(profile, "DevToolsActivePort");
  for (let i = 0; i < 100 && !existsSync(portFile); i++) await sleep(100);
  if (!existsSync(portFile)) {
    proc.kill();
    fail("Chrome didn't start");
  }
  const port = readFileSync(portFile, "utf8").split("\n")[0];
  return {
    port,
    // Its profile goes once it has: it writes there as it closes.
    async stop() {
      const gone = new Promise((done) => proc.once("exit", done));
      proc.kill();
      await gone;
      rmSync(profile, { recursive: true, force: true, maxRetries: 10, retryDelay: 100 });
    },
  };
}

// The page's tab, as something to send Chrome's commands to.
async function openTab(port) {
  const tabs = await (await fetch(`http://127.0.0.1:${port}/json`)).json();
  const tab = tabs.find((t) => t.type === "page");
  const socket = new WebSocket(tab.webSocketDebuggerUrl);
  await new Promise((done, failed) => {
    socket.onopen = done;
    socket.onerror = failed;
  });
  let id = 0;
  const waiting = new Map();
  socket.onmessage = (m) => {
    const d = JSON.parse(m.data);
    const w = waiting.get(d.id);
    if (!w) return;
    waiting.delete(d.id);
    d.error ? w.failed(new Error(d.error.message)) : w.done(d.result);
  };
  const send = (method, params = {}) =>
    new Promise((done, failed) => {
      waiting.set(++id, { done, failed });
      socket.send(JSON.stringify({ id, method, params }));
    });
  const evaluate = async (expression) => (await send("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true })).result?.value;
  return { send, evaluate, close: () => socket.close() };
}

// Once the page has drawn what it was asked for, and nothing's moved for
// a moment: its fonts are in, and it says something.
async function settled(tab) {
  for (let i = 0; i < 100; i++) {
    const ready = await tab.evaluate(`document.fonts.status === "loaded" && (document.querySelector("main")?.innerText.trim().length ?? 0) > 40 && !/Opening the workspace/.test(document.body.innerText)`);
    if (ready) break;
    await sleep(100);
  }
  await sleep(1200);
}

async function main() {
  const keep = args.includes("--keep") && args.splice(args.indexOf("--keep"), 1).length > 0;
  const given = flag("--try");
  const out = resolve(flag("--out") ?? join(here, "docs/assets/shots"));
  const wanted = args.includes("all") ? shots.map((s) => s.name) : args.length ? args : shots.filter((s) => s.readme).map((s) => s.name);
  for (const w of wanted) if (!shots.some((s) => s.name === w)) fail(`no shot named ${w}: there's ${shots.map((s) => s.name).join(", ")}, or all`);

  const name = given ?? `shots-${process.pid}`;
  if (!given) {
    console.error("Making a try, seeded with marketing...");
    tryCommand("up", "-n", name, "marketing");
  }
  let chrome;
  try {
    const link = tryCommand("info", "-n", name, "link");
    const origin = new URL(link).origin;
    chrome = await startChrome();
    const tab = await openTab(chrome.port);
    await tab.send("Page.enable");
    await tab.send("Emulation.setDeviceMetricsOverride", { width: size.width, height: size.height, deviceScaleFactor: size.scale, mobile: false });
    // The link signs the browser in.
    await tab.send("Page.navigate", { url: link });
    await settled(tab);
    mkdirSync(out, { recursive: true });
    for (const shot of shots.filter((s) => wanted.includes(s.name))) {
      for (const theme of ["light", "dark"]) {
        await tab.send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: theme }] });
        await tab.send("Page.navigate", { url: origin + shot.path });
        await settled(tab);
        if (shot.show) {
          const found = await tab.evaluate(`(() => {
            const words = ${JSON.stringify(shot.show)};
            const el = [...document.querySelectorAll("main *")].find((e) => e.children.length === 0 && e.textContent.includes(words));
            el?.scrollIntoView({ block: "center" });
            return !!el;
          })()`);
          if (!found) fail(`${shot.name}: nothing on ${shot.path} says "${shot.show}"`);
          await sleep(400);
        }
        // WebP: a fifth of the PNG's size, and the words as sharp.
        const { data } = await tab.send("Page.captureScreenshot", { format: "webp", quality: 90 });
        const file = join(out, `${shot.name}-${theme}.webp`);
        writeFileSync(file, Buffer.from(data, "base64"));
        console.log(file);
      }
    }
    tab.close();
  } finally {
    await chrome?.stop();
    if (!given && !keep) tryCommand("down", name);
    if (!given && keep) console.error(`Left up: scripts/try.sh info -n ${name}`);
  }
}

main().catch((e) => {
  console.error(`shots: ${e.message}`);
  process.exit(1);
});
