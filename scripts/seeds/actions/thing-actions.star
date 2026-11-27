# Things I do to the fixture's things: ask after one, and dust one that's on the shelf (its note says so, once I've said yes).
when = [
    action("Ask after it", on = "frame-fixture:thing", input = {"question": "text", "how": ["quietly", "loudly"]}),
    action("Dust it", on = "frame-fixture:thing", state = "On the shelf*"),
]
may = ["notify", "frame-fixture:things"]

def run(e):
    thing = show(e.on)
    if e.title == "Dust it":
        done = wait(act("frame-fixture:things", [{"thing": e.on, "note": "Dusted, and back on the shelf."}], why = "You asked to dust " + e.on), timeout = "1h")
        if done and done.accepted:
            notify("Dusted %s: %s" % (e.on, thing.head), open = e.on)
    else:
        notify("You asked %s %s: %s" % (e.on, e.input.how, e.input.question), open = e.on)
