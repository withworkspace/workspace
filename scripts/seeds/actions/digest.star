# Today's digest, when I ask for it: what's happened, by kind.
when = action("Write today's digest")
may = ["output", "notify"]

def run(e):
    kinds = ["mission.started", "capsule.lifted", "doc.added", "proposal.made", "pr.opened", "commits.landed"]
    counts = [(k, len(events(k, since = "1d"))) for k in kinds]
    rows = ["| %s | %d |" % (k, n) for k, n in counts]
    output("## Today\n\n| What | How many |\n|---|---|\n" + "\n".join(rows))
    total = 0
    for _, n in counts:
        total += n
    notify("Today's digest: %d things happened" % total, open = "output")
