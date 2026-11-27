# When Brendan draws something new, ask whether it's ready to build, and if it is, propose a mission for it.
when = event("thing.added", kind = "sketch", by = "brendan")
may = ["ask", "propose", "notify"]

def run(e):
    a = ask(
        "Brendan has just drawn %s. Is it ready to build from, or still a rough idea?" % e.title,
        answer = {"ready": "bool", "why": "text"},
        about = [e.thing],
    )
    if a == None or not a.ready:
        return
    p = propose("mission", {"title": "Build " + e.title}, why = a.why)
    r = wait(p, timeout = "3d")
    if r and r.accepted:
        notify("%s is mission %s now" % (e.title, r.result), open = r.result)
