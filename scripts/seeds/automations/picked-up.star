# When Brendan draws something new, tell me once Ana's picked it up, or if nobody has in a day.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["notify"]

def run(e):
    if e.title.startswith("WIP"):
        return
    picked = wait(event("thing.changed", thing = e.thing, by = "ana"), timeout = "1d")
    if picked == None:
        notify("Nobody's picked up %s yet" % e.title, open = e.thing)
    else:
        notify("%s picked up %s" % (picked.by.short, e.title), open = e.thing)
