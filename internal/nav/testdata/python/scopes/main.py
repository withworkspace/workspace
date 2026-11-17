LIMIT = 3
count = "module"


def total(items):
    n = 0
    for item in items:
        n += len(item) + LIMIT
        #    ^ builtin
        #                ^ resolved :LIMIT "in this file"
    squares = [i * i for i in range(n)]
    #          ^ resolved :i "local"
    add = lambda a: a + n
    #               ^ resolved :a "local"
    #                   ^ resolved :n "local"
    return later + add(squares[0])
    #      ^ resolved :later "local"
    later = 1


class Counter:
    count = 0

    def bump(self):
        return count
        #      ^ resolved :count "in this file"


def outer():
    seen = []

    def inner(x):
        seen.append(x)
        # <- resolved :seen "local"
        return helper(x)
        #      ^ resolved :helper "in this file"

    return inner


def helper(x):
    if x is None:
    #       ^ none
        raise ValueError(missing)
        #     ^ builtin
        #                ^ unresolved
    return x


_state = None


def set_state(value):
    global _state
    _state = value
    # <- resolved :_state "in this file"
    for i, (key, val) in enumerate(value):
        print(i, key, val)
        #        ^ resolved :key "local"
        #             ^ resolved :val "local"
    return ", ".join(value)
    #           ^ builtin
