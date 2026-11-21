#!/usr/bin/env node
// The README's pictures, taken again: the web UI of a try seeded with
// scripts/seeds/marketing.sh, light and dark, by shotta. Which pictures
// there are is web/shotta.config.ts's to say (its exports), and how each is
// come to, web/shots/'s.
//
//   node scripts/shots.mjs [export...]      (just shots)
//
// With no names it takes the shots the README shows (the readme export),
// into docs/assets/shots/ as <name>-light.webp and <name>-dark.webp; with
// names, those of the config's exports (all, for every one, which is
// FEATURES.md's too). It makes its own try (scripts/try.sh up marketing) and
// tears it down after.
//
//   --try <name>   take them of a try that's already up (it's left up)
//   --out <dir>    where they go (default docs/assets/shots)
//   --keep         leave the try it made up, and say where it is
//
// It needs pnpm, for the web UI's packages, and the browser shotta drives,
// which is fetched the first time (once a machine, and a version of
// Playwright).
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const web = join(here, "web");
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

// A command of the web UI's packages, run there, saying what it says.
function inWeb(env, ...command) {
  return spawnSync("pnpm", command, { cwd: web, env: { ...process.env, ...env }, stdio: "inherit" }).status;
}

function main() {
  const keep = args.includes("--keep") && args.splice(args.indexOf("--keep"), 1).length > 0;
  const given = flag("--try");
  const out = flag("--out");
  // No name means the README's; all, every export, which shotta takes no name to mean.
  const wanted = args.includes("all") ? [] : args.length ? args : ["readme"];

  if (!existsSync(join(web, "node_modules")) && inWeb({}, "install", "--frozen-lockfile", "--silent") !== 0) fail("pnpm install failed in web/");
  if (inWeb({}, "exec", "playwright", "install", "chromium-headless-shell") !== 0) fail("the browser couldn't be fetched");

  const name = given ?? `shots-${process.pid}`;
  if (!given) {
    console.error("Making a try, seeded with marketing...");
    tryCommand("up", "-n", name, "marketing");
  }
  try {
    // The link says where the try is, and signs the browser in.
    const env = { WS_LINK: tryCommand("info", "-n", name, "link") };
    if (out) env.SHOTS_OUT = resolve(out);
    if (inWeb(env, "exec", "shotta", "export", ...wanted) !== 0) fail("some pictures weren't taken: what shotta said is above");
  } finally {
    if (!given && !keep) tryCommand("down", name);
    if (!given && keep) console.error(`Left up: scripts/try.sh info -n ${name}`);
  }
}

try {
  main();
} catch (e) {
  console.error(`shots: ${e.message}`);
  process.exit(1);
}
