// Measures a try's pages and reads, with its daemon measuring (WS_MEASURE=1):
//
//   WS_MEASURE=1 scripts/try.sh up -n big marketing scale
//   node scripts/measure/crawl.mjs big [out.json]
//   node scripts/measure/report.mjs /tmp/ws-try/big/.ws/measure.jsonl out.json
//
// It opens each page in a headless browser twice: loaded afresh, and gone
// to from the missions as the app goes (pushState), each time waiting till
// no read has been in flight for a while, and noting how long that took
// and what it asked. Then it asks each GET it saw on its own, three times,
// a little apart. Every request is tagged (X-Ws-Measure) with the page or
// round, so the daemon's lines in .ws/measure.jsonl say whose they were.
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repo = join(here, "..", "..");
const require = createRequire(join(repo, "web") + "/");
const { chromium } = require("playwright");

const name = process.argv[2];
const outPath = process.argv[3] ?? "crawl.json";
if (!name) {
  console.error("crawl.mjs <try-name> [out.json]");
  process.exit(2);
}
const info = (key) => execFileSync(join(repo, "scripts/try.sh"), ["info", key, "-n", name], { encoding: "utf8" }).trim();
const port = info("port");
const token = info("token");
const origin = `http://127.0.0.1:${port}`;
const TAG = "X-Ws-Measure";

async function api(path, tag) {
  const res = await fetch(origin + path, { headers: { Authorization: `Bearer ${token}`, [TAG]: tag } });
  if (!res.ok) return null;
  return res.json().catch(() => null);
}

// --- what there is to open ---------------------------------------------------

const ws = await api("/api/workspace", "discover");
const repos = (await api("/api/repos", "discover"))?.repos ?? [];
const pages = ["/", "/team", "/reviews", "/documents", "/conversations", "/artifacts", "/files", "/runs", "/notifications", "/orchestrator", "/settings"];
for (const s of ["appearance", "keyboard", "notifications", "plugins", "process"]) pages.push(`/settings/${s}`);
const missions = (ws?.missions ?? []).map((m) => m.slug);
for (const slug of missions) pages.push(`/missions/${slug}`);
for (const slug of missions.slice(0, 3)) for (const tab of ["brief", "work", "verify", "talk", "decisions", "log", "links"]) pages.push(`/missions/${slug}?tab=${tab}`);
pages.push("/missions/invoice-totals-off-by-a-penny/review/api/tax-rounding");
pages.push("/missions/export-invoices-as-csv?tab=verify&b=b1");
for (const c of (ws?.conversations ?? []).slice(0, 3)) pages.push(`/conversations/${c.id}`);
for (const a of (ws?.artifacts ?? []).slice(0, 2)) pages.push(`/artifacts/${a.id}`);
pages.push("/reviews/api/41", "/reviews/web/27");
pages.push("/documents/records", "/documents/records/adr/0001-money-is-whole-minor-units.md");
for (const r of repos.filter((r) => ["api", "web", "svc-05", "svc-20"].includes(r.name))) {
  pages.push(`/files/${r.name}`);
  const capsule = (r.capsules ?? []).find((n) => n.startsWith("work-"));
  if (capsule) {
    pages.push(`/files/${r.name}/${capsule}`);
    if (r.name === "api") pages.push(`/missions/paginate-the-invoices-endpoint/files/api/paginate`);
  }
}
const unique = [...new Set(pages)];
console.error(`${unique.length} pages`);

// --- the browser ------------------------------------------------------------

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
await page.goto(`${origin}/auth?next=%2F&t=${token}`);
await page.waitForLoadState("networkidle").catch(() => {});

let inflight = new Map();
let seen = [];
const isRead = (url) => url.startsWith(origin + "/api/") && !url.includes("/api/socket");
page.on("request", (r) => {
  if (!isRead(r.url())) return;
  inflight.set(r, performance.now());
});
const finish = (r, failed) => {
  const t0 = inflight.get(r);
  if (t0 === undefined) return;
  inflight.delete(r);
  seen.push({ method: r.method(), url: r.url().slice(origin.length), ms: Math.round(performance.now() - t0), failed });
};
page.on("requestfinished", (r) => finish(r, false));
page.on("requestfailed", (r) => finish(r, true));

// quiet waits till no read has been in flight for quietFor, and says when
// the last one ended, from t0.
async function quiet(t0, quietFor = 500, most = 20000) {
  let last = performance.now();
  while (performance.now() - t0 < most) {
    if (inflight.size > 0) last = performance.now();
    else if (performance.now() - last >= quietFor) break;
    await new Promise((r) => setTimeout(r, 20));
  }
  return Math.round(last - t0);
}

const results = [];
for (const how of ["load", "go"]) {
  for (const path of unique) {
    const tag = `${how}:${path}`;
    await context.setExtraHTTPHeaders({ [TAG]: tag });
    if (how === "go") {
      // From the missions, as the app goes there: pushState and popstate.
      await context.setExtraHTTPHeaders({ [TAG]: "go:from" });
      await page.goto(origin + "/");
      await quiet(performance.now());
      await context.setExtraHTTPHeaders({ [TAG]: tag });
    }
    inflight = new Map();
    seen = [];
    const t0 = performance.now();
    if (how === "load") await page.goto(origin + path).catch(() => {});
    else await page.evaluate((p) => { history.pushState(null, "", p); dispatchEvent(new PopStateEvent("popstate")); }, path);
    const settled = await quiet(t0);
    const longTasks = await page.evaluate(() => performance.getEntriesByType("longtask")?.length ?? 0).catch(() => 0);
    results.push({ how, path, settledMs: settled, reads: seen.length, slowest: [...seen].sort((a, b) => b.ms - a.ms).slice(0, 3), longTasks, all: seen });
    console.error(`${how.padEnd(4)} ${String(settled).padStart(6)} ms ${String(seen.length).padStart(3)} reads  ${path}`);
  }
}
await browser.close();

// --- each read on its own ------------------------------------------------------

const gets = [...new Set(results.flatMap((r) => r.all.filter((s) => s.method === "GET").map((s) => s.url)))];
console.error(`replaying ${gets.length} reads`);
const replay = [];
for (const url of gets) {
  for (let i = 1; i <= 3; i++) {
    const t0 = performance.now();
    const res = await fetch(origin + url, { headers: { Authorization: `Bearer ${token}`, [TAG]: `replay${i}` } }).catch(() => null);
    const body = res ? await res.arrayBuffer().catch(() => null) : null;
    replay.push({ url, round: i, status: res?.status ?? 0, ms: Math.round(performance.now() - t0), bytes: body?.byteLength ?? 0 });
    await new Promise((r) => setTimeout(r, 250));
  }
}

writeFileSync(outPath, JSON.stringify({ origin, pages: results, replay }, null, 2));
console.error(`wrote ${outPath}`);
