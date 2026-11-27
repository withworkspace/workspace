# Run coverage once a day on every repo that has it, and show the trend over time.
when = every("day 03:00")
may = ["run", "output", "notify"]

def run(e):
    targets = [r for r in repos() if "coverage" in r.tasks]
    started = [run_task("coverage", repo = r.name) for r in targets]
    results = wait_all(started, timeout = "1h")
    today = []
    for res in results:
        if res and res.passed:
            keep(res.repo, res.reports.coverage.total)
            today.append("- **%s** %s%%" % (res.repo, res.reports.coverage.total))
        elif res:
            today.append("- %s: %s" % (res.repo, res.status))
    series = {r.name: kept(r.name, since = "90d") for r in targets}
    output(chart(series, title = "Coverage, 90 days", unit = "%") + "\nToday:\n\n" + "\n".join(today))
    notify("Coverage today: " + ", ".join([l.lstrip("- ").replace("**", "") for l in today]), open = "output")
