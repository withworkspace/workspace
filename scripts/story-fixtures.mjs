#!/usr/bin/env node
// What the web UI's stories draw, taken again: missions as ws's API gives
// them, from tries seeded for it.
//
//   node scripts/story-fixtures.mjs      (just story-fixtures)
//
// A story of a mission's banner is only worth looking at if the mission is
// one ws would give: its headlines as mission.Resolve worked them out, its
// tasks and its plan's stages beside them. So the stories read missions
// captured here, never ones written by hand. It makes two tries, since the
// marketing seed makes a try's workspace over:
//
//   sorrel   marketing: missions in every court
//   demo     mission, work, waits, checks-unmet, and what's set up below
//
// and for each writes, under web/src/mission/fixtures/<try>/:
//
//   workspace.json        GET /api/workspace
//   repos.json            GET /api/repos
//   missions/<slug>.json  GET /api/missions/<slug>
//   talks/<slug>.<as>.json GET /api/missions/<slug>/conversations/<id>
//                          (a mission's planner's conversation, for the
//                          stories of its Brief tab: see briefed below)
//
// and fixtures/court.json: the cards of every case in the court's table of
// tests, which has each kind of card, most of which no seed reaches.
//
// (where the try was on this machine written as /try/<name>, so nothing
// of the machine is kept), with when they were taken in fixtures/taken.json, which the stories set
// their clock by ("2m ago" stays 2m ago), and every kind of headline there
// is in fixtures/kinds.json (from internal/mission/court.go, most pressing
// first), so the stories can say which they don't have. Then it tears the
// tries down.
//
// The demo try is a test workspace, whose agents are Claude Code on haiku:
// its orchestrator is a real one, and may act on a mission between its
// seed and its being taken (it has asked about work-across-repos's Review
// step, which ws had already asked of you). What's taken is still what ws
// gave, but it isn't the same every time.
//
// A state worth a story that no seed leaves a mission in is set up in
// `also` below, with ws's own commands, so it's still ws that says how the
// mission stands. One worth more than a look is a seed.
//
//   --keep   leave the tries up, and say where they are
import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const out = join(here, "web/src/mission/fixtures");
const keep = process.argv.includes("--keep");

const tries = [
  { name: "sorrel", seeds: ["marketing"], also: briefed },
  { name: "demo", seeds: ["mission", "work", "waits", "checks-unmet"], also },
];

// up is the tries made so far: torn down when it's done, and when it fails.
const up = [];

function fail(why) {
  console.error(`story-fixtures: ${why}`);
  if (!keep) for (const name of up.splice(0)) spawnSync(join(here, "scripts/try.sh"), ["down", name], { stdio: "ignore" });
  process.exit(1);
}

function tryCommand(...a) {
  const r = spawnSync(join(here, "scripts/try.sh"), a, { encoding: "utf8", stdio: ["ignore", "pipe", "inherit"] });
  if (r.status !== 0) fail(`scripts/try.sh ${a.join(" ")} failed`);
  return r.stdout.trim();
}

// briefed is what the sorrel try is left in beyond its seed, once its
// missions are taken as marketing left them: the mission it leaves to
// brief, briefed (the briefing seed: its planner has asked, and proposed
// the brief twice, the second time with its plan). It's taken again under
// a second name, with its planner's conversation, which is where what's
// proposed is: what a mission's Brief tab draws.
function briefed(ws, dir, t) {
  const slug = "archive-invoices-older-than-seven-years";
  tryCommand("seed", "-n", t.fullName, "briefing");
  return { changed: [[slug, "briefed"]], made: [], talks: [[slug, "t1", "briefed"]] };
}

// also is what the demo try is left in beyond its seeds, once its
// missions are taken as the seeds left them: the orchestrator asking at a
// Wait ws watches and at one nobody does, and missions at Z, with an
// ending suggested (and a question still open, or none), ended, and
// archived. It returns what to take after: missions it changed, each
// under a second name (<slug>.<as>), and missions it made.
function also(ws, dir) {
  const run = (...a) => {
    const r = spawnSync(ws, a, { cwd: dir, encoding: "utf8" });
    if (r.status !== 0) fail(`ws ${a.join(" ")} failed in ${dir}:\n${r.stderr}`);
  };
  const orchestrator = (slug, ...a) => run(`WS_MISSION=${slug}`, "WS_ROLE=orchestrator", "mission", ...a);
  orchestrator("on-the-registry", "flag", "The release workflow's last run failed on a flaky upload. Re-run it, or wait for the nightly?", "--option", "Re-run it", "--option", "Wait for the nightly");
  orchestrator("platform-deploy", "flag", "The platform team's change window moved to Thursday. Wait for it, or ask them to deploy today?", "--option", "Wait for Thursday", "--option", "Ask them today");

  // finding is a mission with nothing to build, at Z: its one step, a
  // Review, passed.
  const finding = (title, slug) => {
    run("mission", "new", title, "--brief", `## Goal\n${title}, and say.\n\n## Acceptance\n- It's named, with what shows it.\n`);
    const plan = spawnSync(ws, ["mission", "plan", "-m", slug, "--set", "-"], {
      cwd: dir,
      encoding: "utf8",
      input: "## Z\n- It's named, with what shows it.\n## Stops early\n- None.\n## Steps\n1. Review: **Read the finding.** What was found, before it's written up.\n",
    });
    if (plan.status !== 0) fail(`ws mission plan failed for ${slug}:\n${plan.stderr}`);
    run("mission", "accept", "-m", slug);
  };
  const concluded = (slug, text, met) => orchestrator(slug, "conclude", text, "--as", "answered", "--met", `1: ${met}`);
  finding("Which cache is slow", "which-cache-is-slow");
  orchestrator("which-cache-is-slow", "flag", "The token cache and the tenant cache are both slow under load. Name both, or only the slowest?", "--option", "Both", "--option", "Only the slowest");
  concluded("which-cache-is-slow", "The token cache is the slow one: its lookups scan every tenant.", "the profile in the handoff names the token cache");
  finding("Which query is slow", "which-query-is-slow");
  concluded("which-query-is-slow", "The invoices list is the slow one: it reads every line item to total each invoice.", "the query plan in the handoff shows the scan");
  finding("Which index is missing", "which-index-is-missing");
  concluded("which-index-is-missing", "invoices has no index on customer_id, which the list filters by.", "the schema in the handoff shows it");
  run("mission", "end", "-m", "which-index-is-missing");
  finding("Which job is stuck", "which-job-is-stuck");
  concluded("which-job-is-stuck", "The reminder job holds its lock after a failed send.", "the log in the handoff shows the lock held");
  run("mission", "end", "-m", "which-job-is-stuck");
  run("mission", "archive", "which-job-is-stuck");
  return {
    changed: [
      ["on-the-registry", "asked"],
      ["platform-deploy", "asked"],
    ],
    made: ["which-cache-is-slow", "which-query-is-slow", "which-index-is-missing", "which-job-is-stuck"],
  };
}

async function capture(t) {
  const dir = tryCommand("info", "dir", "-n", t.fullName);
  const port = tryCommand("info", "port", "-n", t.fullName);
  const token = tryCommand("info", "token", "-n", t.fullName);
  const get = async (path) => {
    const res = await fetch(`http://127.0.0.1:${port}${path}`, { headers: { authorization: `Bearer ${token}`, accept: "application/json" } });
    if (!res.ok) fail(`GET ${path} on ${t.fullName}: ${res.status}`);
    return res.json();
  };
  const to = join(out, t.name);
  mkdirSync(join(to, "missions"), { recursive: true });
  // Nothing of this machine is kept: where the try was is written as
  // /try/<name>, and the build it ran as "stories".
  const there = [dir, dir.replace(/^\/private/, ""), `/private${dir}`];
  const plain = (data) => {
    let text = JSON.stringify(data, null, 2);
    for (const d of there) text = text.split(d).join(`/try/${t.name}`);
    return text.replace(/("version": )"[^"]*"/g, '$1"stories"');
  };
  const write = (file, data) => writeFileSync(join(to, file), plain(data) + "\n");
  const mission = async (slug, name = slug) => write(`missions/${name}.json`, await get(`/api/missions/${encodeURIComponent(slug)}`));
  let n = 0;
  for (const m of (await get("/api/workspace")).missions) {
    await mission(m.slug);
    n++;
  }
  if (t.also) {
    const { changed, made, talks = [] } = t.also(join(dir, ".try/ws"), dir, t);
    for (const [slug, as] of changed) await mission(slug, `${slug}.${as}`);
    for (const slug of made) await mission(slug);
    n += changed.length + made.length;
    // A conversation of a mission's, under the name the mission was taken as.
    if (talks.length) mkdirSync(join(to, "talks"), { recursive: true });
    for (const [slug, id, as] of talks) write(`talks/${slug}.${as}.json`, await get(`/api/missions/${encodeURIComponent(slug)}/conversations/${encodeURIComponent(id)}`));
  }
  // The workspace as it ends up, with every mission the stories have.
  write("workspace.json", await get("/api/workspace"));
  write("repos.json", await get("/api/repos"));
  return n;
}

// kinds is every kind of headline, as court.go orders them: its constants'
// values, in kindOrder's order.
function kinds() {
  const src = readFileSync(join(here, "internal/mission/court.go"), "utf8");
  const value = new Map([...src.matchAll(/^\s*(Kind\w+)\s+Kind = "([a-z-]+)"/gm)].map((m) => [m[1], m[2]]));
  const order = /var kindOrder = \[\]Kind\{([^}]*)\}/.exec(src);
  if (!order) fail("internal/mission/court.go has no kindOrder to read");
  return [...order[1].matchAll(/Kind\w+/g)].map((m) => value.get(m[0]) ?? fail(`court.go's ${m[0]} has no value`));
}

// ws is built first, for the tries below, while the fixtures there are
// still there: the web UI's own code reads some of them (a sample
// workspace, a test's mission), and doesn't build without them.
if (spawnSync("just", ["build"], { cwd: here, stdio: ["ignore", 2, 2] }).status !== 0) fail("just build failed");

rmSync(out, { recursive: true, force: true });
mkdirSync(out, { recursive: true });
writeFileSync(join(out, "kinds.json"), JSON.stringify(kinds(), null, 2) + "\n");

// Every case of the court's table (internal/mission/court_test.go), as the
// cards ws makes of it: each kind of card there is, where no seed leaves
// a mission.
const court = spawnSync("go", ["test", "./internal/mission", "-count=1", "-run", "TestResolve_EveryMissionIsInOneCourt|TestCourtCardsForStories"], {
  cwd: here,
  encoding: "utf8",
  env: { ...process.env, WS_STORY_FIXTURES: "1" },
});
if (court.status !== 0) fail(`the court's cards weren't written:\n${court.stdout}${court.stderr}`);
for (const t of tries) {
  t.fullName = `stories-${t.name}`;
  tryCommand("up", "-n", t.fullName, "--no-build", ...t.seeds);
  up.push(t.fullName);
  const n = await capture(t);
  console.error(`${t.name}: ${n} missions`);
}
writeFileSync(join(out, "taken.json"), JSON.stringify({ at: new Date().toISOString() }, null, 2) + "\n");
if (keep) {
  for (const t of tries) console.error(`Left up: scripts/try.sh info -n ${t.fullName}`);
} else {
  for (const t of tries) tryCommand("down", t.fullName);
}
