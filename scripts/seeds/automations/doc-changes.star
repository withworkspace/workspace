# Tell me when a decision changes in records, saying who and what.
when = event("doc.changed", repo = "records", path = "decisions/*")
may = ["notify"]

def run(e):
    notify("%s %s %s" % (e.by.short, e.status, e.title), open = e.doc)
