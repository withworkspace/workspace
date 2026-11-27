# When one of the fixture's things is moved, note where it went, without asking me.
when = event("thing.changed", kind = "frame-fixture:thing", state = "*")
may = ["frame-fixture:things"]

def run(e):
    to = [c.to for c in e.changes if c.field == "state"]
    if to:
        act("frame-fixture:things", [{"thing": e.thing, "note": "Moved: " + to[0] + "."}], why = "It moved")
