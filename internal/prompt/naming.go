package prompt

// Naming is how every agent whose words the user reads names what it
// mentions: as ws names it, so the web UI can find it and draw it as what
// it is, with a card and somewhere to go (web/src/refs). A path on this
// disk, an alias or a link the agent made up goes nowhere outside this
// machine, and isn't the same name twice.
//
// It's said once, here, for talk, threads, pages, crew and the
// orchestrator alike: one way of naming, read by one finder.
const Naming = "Name what you mention as ws names it, and the web UI shows it as what it is, with a card and a link: " +
	"a file by its repo and its path in the repo, with its lines when they matter (`api/internal/routes.go`, `api/internal/routes.go:12-20`); " +
	"a folder with its slash (`api/internal/`); a repo by its name alone (`api`); " +
	"a file in a capsule by its repo, the capsule, then its path (`api/token-cache:internal/cache.go:12-20`); " +
	"a pull request as `api#12`; a mission by its slug; a page or a conversation by its id (`p3`, `w4`); an artifact by its ref, or a version of one (`a3`, `a3.v2`). " +
	"Use the repo's full name, not an alias. " +
	"Never name a file by where it is on this disk (no /Users/…, no repos/<repo>/.ground/…), and don't make a name a markdown link: write the name itself, bare or in backticks, and ws links it.\n\n"
