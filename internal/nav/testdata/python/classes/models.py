from typing import Optional


class Store:
    def get(self, key):
        return key


class Base:
    def describe(self) -> str:
        return "base"


class Server(Base):
    """Serves things."""

    def __init__(self, store: "Store", name: str):
        self.store = store
        self.name = name

    def size(self) -> int:
        return len(self.store.get(self.name))
        #               ^ resolved :store "a member of Server"
        #                     ^ resolved :get "by the type written on self.store"
        #                              ^ resolved :name

    def label(self) -> str:
        return self.describe() + super().describe()
        #           ^ resolved :describe
        #                                ^ resolved :describe

    @classmethod
    def make(cls) -> "Server":
        return cls(Store(), "x")


def run(maybe: Optional[Server]):
    made = Server.make()
    made.size()
    #    ^ resolved :size "by the type written on made"
    maybe.label()
    #     ^ resolved :label "by the type written on maybe"
    s = Store()
    s.get("k")
    # ^ resolved :get "by the type written on s"
