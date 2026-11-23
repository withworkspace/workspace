// The scenes looks.mjs goes through: a page of a try, and what's pressed
// on it to bring more into view (looks.mjs says what a step can be). They
// are of the marketing seed, with tweaks, briefing, files and
// plugin-proposals after it:
//
//   just try up marketing tweaks briefing files plugin-proposals
//
// A scene changes nothing: it opens a dialog, and leaves it unsent (so
// none presses New page, which makes one). One that finds nothing is said
// when they're run: add a scene for what a change touches that none of
// these reach, and say which seed it needs.

const button = (name) => `role=button[name=/${name}/]`;
const at = (name, path, ...steps) => ({ name, path, steps });

const penny = "/missions/invoice-totals-off-by-a-penny";
const csv = "/missions/export-invoices-as-csv";
const currency = "/missions/invoices-in-more-than-one-currency";
const adr = "/documents/records/adr/0001-money-is-whole-minor-units.md";

export const scenes = [
  // The window.
  at("find", "/", { key: "Meta+k" }),
  at("find, asked", "/", { key: "Meta+k" }, { type: "inv" }),

  // Missions.
  at("missions", "/"),
  at("mission", penny),
  at("mission: rename", penny, { click: button("^Rename the mission") }),
  at("mission: send back", penny, { click: button("^Send back") }),
  at("mission: end", penny, { click: button("^End$") }),
  at("mission: log", `${penny}?tab=log`),
  at("mission: talk", `${penny}?tab=talk`),
  at("mission: brief", `${currency}?tab=brief`),
  at("mission: brief, a link", `${currency}?tab=brief`, { click: ".ProseMirror p" }, { click: button("^Link") }),
  at("mission: brief, as markdown", `${currency}?tab=brief`, { click: button("^Markdown$") }),
  at("mission: work", `${csv}?tab=work`),
  at("mission: open a pull request", `${csv}?tab=work`, { click: button("^Open pull request") }),
  at("mission: hand a capsule on", `${csv}?tab=work`, { click: button("^Hand on") }),
  at("mission: drop a capsule", `${csv}?tab=work`, { click: button("^Drop") }),
  at("mission: verify", `${csv}?tab=verify&b=b1`),
  at("mission: verify, not right", `${csv}?tab=verify&b=b1`, { click: button("^Not right") }),
  at("mission: a question from crew", "/missions/rate-limit-the-public-api"),
  at("mission: ready", "/missions/show-vat-by-rate-on-the-pdf"),
  at("crew", "/missions/dark-mode-for-the-dashboard/crew/c1"),

  // A mission's review.
  at("review", `${penny}/review/api/tax-rounding`),
  at("review: changes", `${penny}/review/api/tax-rounding`, { click: button("^Changes") }),
  at("review: jump to a file", `${penny}/review/api/tax-rounding`, { key: "t" }),
  at("review: submit", `${penny}/review/api/tax-rounding`, { click: button("^Submit review") }),

  // Pull requests.
  at("reviews", "/reviews"),
  at("pull request", "/reviews/api/41"),
  at("pull request: discussion", "/reviews/api/41", { click: button("^Discussion") }),
  at("pull request: changes", "/reviews/api/41", { click: button("^Changes") }),
  at("pull request: submit", "/reviews/api/41", { click: button("^Submit review") }),

  // Conversations.
  at("conversations", "/conversations"),
  at("conversation", "/conversations/w1"),
  at("conversation: a proposal, edited", "/conversations/w1", { click: button("^Edit$") }),
  at("conversation: details", "/conversations/w1", { click: button("^Details") }),
  at("conversation w2", "/conversations/w2"),
  at("conversation w3", "/conversations/w3"),

  // Documents and pages.
  at("documents", "/documents"),
  at("documents: new as", "/documents", { click: button("^New as") }),
  at("space", "/documents/records"),
  at("space: new", "/documents/records", { click: button("^New") }),
  at("document", adr),
  at("document: asked about", adr, { click: button("^Ask") }),
  at("page", `${adr}?edit`),
  at("page: add", `${adr}?edit`, { click: button("^Add$") }),
  at("page: change", `${adr}?edit`, { click: ".ProseMirror p" }, { click: button("^Change") }),
  at("page: save to the repo", `${adr}?edit`, { click: button("^Save to repo") }),
  at("page: a pass", `${adr}?edit`, { click: "role=tab[name=/^Check/]" }),

  // Artifacts.
  at("artifacts", "/artifacts"),
  at("artifact", "/artifacts/a1"),
  at("artifact: promote", "/artifacts/a1", { click: button("^Promote") }),
  at("artifact: promote, into a mission", "/artifacts/a1", { click: button("^Promote") }, { click: 'role=dialog >> button[aria-label="What it went into"]' }, { click: "role=option[name=/^A mission/]" }),
  at("artifact: promote, into a new mission", "/artifacts/a1", { click: button("^Promote") }, { click: 'role=dialog >> button[aria-label="What it went into"]' }, { click: "role=option[name=/^A new mission/]" }),
  at("artifact: promote, into a document", "/artifacts/a1", { click: button("^Promote") }, { click: 'role=dialog >> button[aria-label="What it went into"]' }, { click: "role=option[name=/^A document/]" }),
  at("artifact: promote, living elsewhere", "/artifacts/a1", { click: button("^Promote") }, { click: `role=dialog >> button[aria-label="Where it's to live"]` }, { click: "role=option[name=/^Another folder/]" }),
  at("artifact: discard", "/artifacts/a1", { click: button("^Discard") }),
  at("artifact: tweaks", "/artifacts/a2/v1"),
  at("artifact: tweaks, second version", "/artifacts/a2"),
  at("artifact: files", "/artifacts/a3"),
  at("artifact: compared", "/artifacts/a2/compare?v=1,2"),

  // Repos and files.
  at("repos", "/files"),
  at("files", "/files/api"),
  at("files: find a file", "/files/api", { key: "t" }),
  at("files: search the files", "/files/api", { key: "Meta+Shift+f" }),
  at("files: a file", "/files/web?path=src%2Finvoices%2Fmoney.ts"),

  // The rest of the sidebar.
  at("team", "/team"),
  at("orchestrator", "/orchestrator"),
  at("runs", "/runs"),

  // Settings.
  ...["appearance", "notifications", "agents", "missions", "team", "repos", "runs", "process", "plugins", "yours", "about"].map((s) => at(`settings: ${s}`, `/settings/${s}`)),
  at("settings: found", "/settings", { click: 'input[placeholder^="Find a setting"]' }, { type: "model" }),
  at("settings: add a repo", "/settings/repos", { click: button("^Add repo") }),
];
