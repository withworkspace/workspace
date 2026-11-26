"""greet: says hello to whoever it's told."""

import sys


def greeting(name):
    return "Hello, " + name


if __name__ == "__main__":
    print(greeting(sys.argv[1]))
