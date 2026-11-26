# Once a sketch's been quiet for a minute, tell me how many saves it had.
when = event("thing.changed", kind = "sketch", settle = "1m", per = "thing")
may = ["notify"]

def run(e):
    notify("%s saved %s %d times" % (e.by.short, e.title, len(e.batch)), open = e.thing)
