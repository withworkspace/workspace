import { beforeEach, describe, expect, it } from "vitest";
import { resting } from "./chromeState";
import { besideNow, useChromeStore } from "./store";

// The store, as anything in the window uses it: by its actions, with no
// component between.
const store = () => useChromeStore.getState();
const beside = () => besideNow(store().chrome, store().here, store().gives);

beforeEach(() => useChromeStore.setState({ root: "", chrome: resting, here: undefined, gives: {}, keys: {} }));

describe("opening something beside", () => {
  it("about the page it was asked on puts it beside that page only", () => {
    store().setHere("document:records:adr/3.md");
    store().open("/conversations/x", { here: true });
    expect(beside()).toMatchObject({ shown: true, here: ["/conversations/x"], front: "/conversations/x", about: ["/conversations/x"] });
    store().setHere("mission:dark-mode");
    expect(beside()).toMatchObject({ shown: false, all: [], about: [] });
    store().setHere("document:records:adr/3.md");
    expect(beside()).toMatchObject({ shown: true, front: "/conversations/x" });
  });

  it("with the keys asks its pane to take them, once", () => {
    store().open("/conversations/x", { keys: true });
    expect(store().keys["/conversations/x"]).toBe(1);
    expect(store().takeKeys("/conversations/x")).toBe(true);
    expect(store().takeKeys("/conversations/x")).toBe(false);
  });

  it("closed, is gone, and what it was asked is forgotten", () => {
    store().open("/conversations/x", { keys: true });
    store().close("/conversations/x");
    expect(beside().all).toEqual([]);
    expect(store().keys).toEqual({});
  });
});

describe("a conversation that hasn't begun", () => {
  it("is there only while the page it's about says how asking begins it", () => {
    store().setHere("thing:fixture/THING-2");
    store().open("/ask/THING-2", { here: true });
    expect(beside().all).toEqual([]);
    const gone = store().give("/ask/THING-2", { start: () => Promise.reject(new Error("unused")) });
    expect(beside().all).toEqual(["/ask/THING-2"]);
    gone();
    expect(beside().all).toEqual([]);
  });

  it("gives its place to the conversation it becomes", () => {
    store().setHere("thing:fixture/THING-2");
    store().give("/ask/THING-2", { start: () => Promise.reject(new Error("unused")) });
    store().open("/ask/THING-2", { here: true });
    store().open("/conversations/x", { here: true, inPlaceOf: "/ask/THING-2" });
    expect(beside()).toMatchObject({ all: ["/conversations/x"], front: "/conversations/x" });
  });
});

describe("what a page gives a tab", () => {
  it("is there until the page takes it back, and taking back what's since been given again leaves the newer", () => {
    const first = store().give("/conversations/b", { placeholder: "one" });
    store().give("/conversations/b", { placeholder: "two" });
    first();
    expect(store().gives["/conversations/b"]?.placeholder).toBe("two");
  });
});
