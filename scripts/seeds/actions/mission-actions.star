# Note where a mission's at, on a mission that's under way.
when = action("Note where it's at", on = "mission", state = ["active", "briefing"])
may = ["notify"]

def run(e):
    m = show(e.on)
    notify("%s: %s" % (e.on, m.head), open = e.on)
