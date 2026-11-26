import { describe, expect, it } from "vitest";
import type { InstalledPlugin } from "../api";
import { addsOf, changedSaid, fromOf, home, outCount, stateOf, updateOf, wantedFrom } from "./installed";

const plugin = (over: Partial<InstalledPlugin> = {}): InstalledPlugin => ({ name: "jira", state: "ready", from: {}, newer: false, ...over });

describe("where a plugin stands", () => {
  it("is trouble only when it's failed or wants setting up", () => {
    expect(stateOf(plugin({ state: "ready" })).tone).toBe("done");
    expect(stateOf(plugin({ state: "stopped" })).tone).toBe("quiet");
    expect(stateOf(plugin({ state: "setup" })).tone).toBe("attention");
    expect(stateOf(plugin({ state: "failed" })).tone).toBe("danger");
  });
});

describe("where a plugin's from", () => {
  it("names the repo and the release", () => {
    expect(fromOf(plugin({ from: { repo: "acme/plugin-jira", tag: "v0.3.0" } }))).toBe("acme/plugin-jira v0.3.0");
  });
  it("says one was built here, at a tag or not", () => {
    expect(fromOf(plugin({ from: { repo: "acme/plugin-jira", built: true } }))).toBe("acme/plugin-jira, built from source");
    expect(fromOf(plugin({ from: { repo: "acme/plugin-jira", tag: "v0.3.0", built: true } }))).toBe("acme/plugin-jira v0.3.0, built from source");
  });
  it("says a link's folder, from your home", () => {
    expect(fromOf(plugin({ from: { linked: "/Users/sam/code/plugin-jira" } }))).toBe("Linked to ~/code/plugin-jira");
    expect(home("/home/sam")).toBe("~");
    expect(home("/opt/plugins/jira")).toBe("/opt/plugins/jira");
  });
  it("says a copy is from a folder", () => {
    expect(fromOf(plugin())).toBe("Copied from a folder on this machine");
  });
});

describe("what a plugin adds", () => {
  it("is each thing in a few words", () => {
    expect(addsOf(plugin({ views: ["Board"], commands: "jira", guide: true, names: ["RAM-…", "OPS-…"] }))).toEqual([
      "Board in the sidebar",
      "ws jira",
      "a guide for agents",
      "names RAM-…, OPS-…",
    ]);
  });
  it("is nothing for one that's never said", () => {
    expect(addsOf(plugin())).toEqual([]);
  });
});

describe("a plugin's update", () => {
  it("isn't offered for one from a folder", () => {
    expect(updateOf(plugin()).can).toBe(false);
    expect(updateOf(plugin({ from: { linked: "/x" } })).can).toBe(false);
  });
  it("names the release that's out", () => {
    const u = updateOf(plugin({ from: { repo: "acme/plugin-jira", tag: "v0.3.0" }, latest: "v0.4.0", newer: true }));
    expect(u).toMatchObject({ can: true, out: true, label: "Update to v0.4.0" });
  });
  it("looks, where none is known to be out", () => {
    const u = updateOf(plugin({ from: { repo: "acme/plugin-jira", tag: "v0.3.0" }, latest: "v0.3.0" }));
    expect(u).toMatchObject({ can: true, out: false, label: "Check for updates" });
  });
  it("fetches one built from source again", () => {
    expect(updateOf(plugin({ from: { repo: "acme/plugin-jira", built: true } })).label).toBe("Rebuild");
  });
  it("is counted across the plugins", () => {
    expect(outCount([plugin({ newer: true }), plugin(), plugin({ newer: true })])).toBe(2);
  });
});

describe("what a change did", () => {
  it("says what was added, and what it still needs", () => {
    expect(changedSaid({ name: "jira", version: "0.3.0" }, "added")).toBe("Added jira 0.3.0");
    expect(changedSaid({ name: "jira", missing: ["site", "token"] }, "added")).toBe("Added jira. Needs setting up: site, token");
  });
  it("says an update from and to", () => {
    expect(changedSaid({ name: "jira", was: "v0.3.0", now: "v0.4.0" }, "updated")).toBe("jira: v0.3.0 → v0.4.0");
  });
  it("says when there was nothing later", () => {
    expect(changedSaid({ name: "jira", now: "v0.3.0", current: true }, "updated")).toBe("jira v0.3.0 is the latest");
  });
});

describe("one of the team's", () => {
  it("is added at the release the team names", () => {
    expect(wantedFrom({ name: "jira", source: "acme/plugin-jira", version: "v0.3.0" })).toBe("acme/plugin-jira@v0.3.0");
    expect(wantedFrom({ name: "jira", source: "acme/plugin-jira" })).toBe("acme/plugin-jira");
    expect(wantedFrom({ name: "jira", source: "acme/plugin-jira@v0.2.0", version: "v0.3.0" })).toBe("acme/plugin-jira@v0.2.0");
  });
});
