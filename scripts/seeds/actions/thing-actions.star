# Things I do to the fixture's things: ask after one, and dust one that's on the shelf.
when = [
    action("Ask after it", on = "frame-fixture:thing", input = {"question": "text", "how": ["quietly", "loudly"]}),
    action("Dust it", on = "frame-fixture:thing", state = "On the shelf*"),
]
may = ["notify"]

def run(e):
    thing = show(e.on)
    if e.title == "Dust it":
        notify("Dusted %s: %s" % (e.on, thing.head), open = e.on)
    else:
        notify("You asked %s %s: %s" % (e.on, e.input.how, e.input.question), open = e.on)
