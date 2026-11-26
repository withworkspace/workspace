# Notify me if Brendan pushes a new drawing.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["notify"]

def run(e):
    log("heard " + e.ref + ": " + e.thing)
    notify("%s drew %s" % (e.by.short, e.title), open = e.thing)
