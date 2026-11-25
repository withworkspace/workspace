// Reads a daemon's .ws/measure.jsonl (and crawl.mjs's output, if given)
// and says, per route, what a request cost: time, git, SQL, gh, RPC, what
// was left, and where the long ones waited; then what the daemon did
// behind the requests.
//
//   node scripts/measure/report.mjs <measure.jsonl> [crawl.json] [--from <line>]
import { readFileSync } from "node:fs";

const args = process.argv.slice(2);
const from = args.includes("--from") ? Number(args[args.indexOf("--from") + 1]) : 0;
const files = args.filter((a, i) => !a.startsWith("--") && args[i - 1] !== "--from");
const [logPath, crawlPath] = files;
const lines = readFileSync(logPath, "utf8").split("\n").slice(from).filter(Boolean).map((l) => JSON.parse(l));
const reqs = lines.filter((l) => l.pattern !== undefined && l.tag !== "discover");
const behind = lines.filter((l) => l.behind_s);

const pct = (xs, p) => {
  if (!xs.length) return 0;
  const s = [...xs].sort((a, b) => a - b);
  return s[Math.min(s.length - 1, Math.floor((p / 100) * s.length))];
};
const r1 = (x) => (x >= 100 ? Math.round(x) : Math.round(x * 10) / 10);
const cost = (l, k) => l.costs?.[k] ?? { n: 0, ms: 0 };
const sqlMs = (l) => cost(l, "sql_read").ms + cost(l, "sql_write").ms + cost(l, "sql_turn").ms;
const sqlTx = (l) => cost(l, "sql_read").n + cost(l, "sql_write").n;

function table(rows, head) {
  const w = head.map((h, i) => Math.max(h.length, ...rows.map((r) => String(r[i]).length)));
  const line = (r) => "| " + r.map((c, i) => String(c).padEnd(w[i])).join(" | ") + " |";
  return [line(head), "|" + w.map((n) => "-".repeat(n + 2)).join("|") + "|", ...rows.map(line)].join("\n");
}

// --- per route ------------------------------------------------------------------

const byRoute = new Map();
for (const l of reqs) {
  const k = l.pattern || `${l.method} ${l.path.split("?")[0]}`;
  if (!byRoute.has(k)) byRoute.set(k, []);
  byRoute.get(k).push(l);
}
const waitsOf = (ls) => {
  const m = new Map();
  for (const l of ls) for (const [w, ms] of Object.entries(l.waits ?? {})) m.set(w, (m.get(w) ?? 0) + ms);
  return [...m].sort((a, b) => b[1] - a[1]);
};
const routes = [...byRoute].map(([k, ls]) => {
  const ms = ls.map((l) => l.ms);
  const top = waitsOf(ls).filter(([w]) => !w.startsWith("running") && !w.startsWith("runnable"))[0];
  return {
    k, n: ls.length, p50: pct(ms, 50), max: Math.max(...ms),
    git: Math.max(...ls.map((l) => cost(l, "git").n)), gitMs: Math.max(...ls.map((l) => cost(l, "git").ms)),
    tx: Math.max(...ls.map(sqlTx)), stmts: Math.max(...ls.map((l) => l.stmts ?? 0)), sql: Math.max(...ls.map(sqlMs)),
    turn: Math.max(...ls.map((l) => cost(l, "sql_turn").ms)),
    gh: Math.max(...ls.map((l) => cost(l, "gh").n)), rpc: Math.max(...ls.map((l) => cost(l, "rpc").n)),
    http: Math.max(...ls.map((l) => cost(l, "http").n)),
    rest: Math.max(...ls.map((l) => l.rest_ms ?? 0)), kb: Math.max(...ls.map((l) => l.bytes)) / 1024,
    wait: top ? `${top[0]} (${r1(top[1] / ls.length)} ms avg)` : "",
  };
});
routes.sort((a, b) => b.max - a.max);

console.log(`# What a read costs\n\n${reqs.length} requests, ${byRoute.size} routes.\n`);
console.log("## Routes, slowest first\n");
console.log("Each column is the most any one request of the route took or did (n is how many were seen). sql is time in transactions, turn waiting to write; rest is time not in git, SQL, gh, RPC or HTTP; wait is what its goroutine was most often seen waiting on, outside running.\n");
console.log(table(
  routes.map((r) => [r.k, r.n, r1(r.p50), r1(r.max), r.git ? `${r.git} (${r1(r.gitMs)} ms)` : "", r.tx ? `${r.tx}/${r.stmts} (${r1(r.sql)} ms)` : "", r.turn ? r1(r.turn) : "", r.gh || "", r.rpc || "", r.http || "", r1(r.rest), r1(r.kb), r.wait]),
  ["route", "n", "p50 ms", "max ms", "git", "sql tx/stmts", "turn", "gh", "rpc", "http", "rest ms", "KB", "wait"],
));

// --- who asked, for the costliest routes ------------------------------------------

const askers = (ls) => {
  const m = new Map();
  for (const l of ls) for (const b of l.by ?? []) {
    const t = m.get(b.who) ?? { n: 0, ms: 0 };
    t.n += b.n;
    t.ms += b.ms;
    m.set(b.who, t);
  }
  return [...m].sort((a, b) => b[1].ms - a[1].ms);
};
const costly = [...byRoute].map(([k, ls]) => [k, ls, ls.reduce((s, l) => s + (l.ms - (l.rest_ms ?? 0)), 0)]).filter(([, , t]) => t > 20).sort((a, b) => b[2] - a[2]).slice(0, 12);
if (costly.length) {
  console.log("\n## Who asked, on the costliest routes\n\nThe routes whose requests spent most time in git, SQL, gh, RPC or HTTP, in all, and who in ws asked for it (\"kind subcommand · function\"), per request.\n");
  for (const [k, ls] of costly) {
    console.log(`**${k}** (${ls.length} requests)\n`);
    for (const [who, t] of askers(ls).slice(0, 6)) console.log(`- ${who}: ${r1(t.n / ls.length)} × , ${r1(t.ms / ls.length)} ms`);
    console.log();
  }
}

// --- first ask vs again ---------------------------------------------------------

const rounds = reqs.filter((l) => /^replay\d$/.test(l.tag ?? ""));
if (rounds.length) {
  const byUrl = new Map();
  for (const l of rounds) {
    if (!byUrl.has(l.path)) byUrl.set(l.path, {});
    byUrl.get(l.path)[l.tag] = l;
  }
  const rows = [...byUrl]
    .map(([u, r]) => [u, r.replay1, r.replay2, r.replay3])
    .filter(([, a, b, c]) => a && b && c && (Math.max(a.ms, b.ms, c.ms) > 20 || cost(a, "git").n + cost(b, "git").n + cost(c, "git").n > 0))
    .sort((x, y) => Math.max(y[1].ms, y[2].ms, y[3].ms) - Math.max(x[1].ms, x[2].ms, x[3].ms))
    .map(([u, a, b, c]) => [u.length > 90 ? u.slice(0, 87) + "…" : u, ...[a, b, c].map((l) => `${r1(l.ms)} ms${cost(l, "git").n ? ` · ${cost(l, "git").n} git` : ""}${l.stmts ? ` · ${l.stmts} stmts` : ""}`)]);
  console.log("\n## Each read alone, three times\n\nReads over 20 ms or starting git in any round, slowest first.\n");
  console.log(table(rows, ["read", "1st", "2nd", "3rd"]));
}

// --- pages ------------------------------------------------------------------------

if (crawlPath) {
  const crawl = JSON.parse(readFileSync(crawlPath, "utf8"));
  const byTag = new Map();
  for (const l of reqs) if (l.tag) (byTag.get(l.tag) ?? byTag.set(l.tag, []).get(l.tag)).push(l);
  const rows = crawl.pages
    .filter((p) => p.settledMs > 100 || p.reads > 10)
    .sort((a, b) => b.settledMs - a.settledMs)
    .map((p) => {
      const ls = byTag.get(`${p.how}:${p.path}`) ?? [];
      const slow = [...ls].sort((a, b) => b.ms - a.ms)[0];
      return [p.how, p.path, p.settledMs, p.reads, ls.reduce((s, l) => s + cost(l, "git").n, 0), ls.reduce((s, l) => s + (l.stmts ?? 0), 0), slow ? `${slow.pattern} ${r1(slow.ms)} ms` : ""];
    });
  console.log("\n## Pages\n\nTill no read was in flight for 500 ms, loaded afresh (load) or gone to in the app (go). Pages over 100 ms or with more than ten reads.\n");
  console.log(table(rows, ["how", "page", "settled ms", "reads", "git", "stmts", "slowest read"]));
}

// --- behind ---------------------------------------------------------------------------

if (behind.length) {
  const sum = {};
  for (const b of behind) for (const [k, v] of Object.entries(b.costs ?? {})) {
    sum[k] ??= { n: 0, ms: 0 };
    sum[k].n += v.n;
    sum[k].ms += v.ms;
  }
  const secs = behind.reduce((s, b) => s + b.behind_s, 0);
  const stmts = behind.reduce((s, b) => s + (b.stmts ?? 0), 0);
  console.log(`\n## Behind the requests\n\nOver ${secs} s: what no request's goroutine did (the daemon's own work, and what reads left to be done behind them), per minute.\n`);
  console.log(table(
    Object.entries(sum).map(([k, v]) => [k, r1((v.n * 60) / secs), r1((v.ms * 60) / secs)]).concat([["statements", r1((stmts * 60) / secs), ""]]),
    ["kind", "per min", "ms per min"],
  ));
  const all = askers(behind).slice(0, 20);
  if (all.length) {
    console.log("\nWho asked for it, per minute:\n");
    console.log(table(all.map(([who, t]) => [who, r1((t.n * 60) / secs), r1((t.ms * 60) / secs)]), ["who", "per min", "ms per min"]));
  }
  const g = behind.map((b) => b.goroutines);
  const h = behind.map((b) => b.heap_mb);
  console.log(`\nGoroutines ${Math.min(...g)}–${Math.max(...g)}, heap ${r1(Math.min(...h))}–${r1(Math.max(...h))} MB.`);
}
