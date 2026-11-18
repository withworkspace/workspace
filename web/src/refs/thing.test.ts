import { describe, expect, it } from "vitest";
import type { Named } from "./refs";
import { namesIn, thingBlocks, type ThingRecord, viewOf } from "./thing";

const issue: Named = {
  plugin: "jira",
  what: "issue",
  prefixes: ["RAM-"],
  title: "Jira issue",
  fields: [
    { name: "title", type: "title" },
    { name: "status", type: "state", label: "Status" },
    { name: "assignee", type: "text", label: "Assignee" },
    { name: "updated", type: "time", label: "Updated" },
    { name: "points", type: "number", label: "Points" }, // a type ws doesn't know
    { name: "body", type: "markdown" },
  ],
  views: {
    card: [
      { block: "title", field: "title" },
      { block: "subtitle", field: "key" },
      { block: "badge", field: "status" },
      { block: "field", field: "assignee" },
      { block: "markdown", field: "body", lines: 4 },
    ],
  },
};
const record: ThingRecord = {
  key: "RAM-12",
  fields: { title: "Token refresh is flaky", status: "In Progress", assignee: "Dana", updated: "2026-11-18T10:42:00Z", points: "3", body: "The **clock** should be injected." },
};
const said = (named: Named | undefined, surface: "card" | "pane", r = record) => thingBlocks(named, r, surface).map((d) => [d.block, d.field, d.value]);

describe("a plugin's thing, drawn", () => {
  it("is the blocks its plugin declared, each with its field's value", () => {
    expect(thingBlocks(issue, record, "card")).toEqual([
      { block: "title", field: "title", value: "Token refresh is flaky", icon: true },
      { block: "subtitle", field: "key", value: "RAM-12" },
      { block: "badge", field: "status", value: "In Progress", inline: true },
      { block: "field", field: "assignee", value: "Dana", label: "Assignee" },
      { block: "markdown", field: "body", value: "The **clock** should be injected.", lines: 4 },
    ]);
  });

  it("leaves out a block with nothing to show, and one ws doesn't know", () => {
    const named: Named = {
      ...issue,
      views: {
        card: [
          { block: "title", field: "title" },
          { block: "avatar", field: "assignee" }, // from a newer ws
          { block: "subtitle", field: "nothing" }, // not a field it has
          { block: "field", field: "assignee" },
          { block: "markdown", field: "body" },
        ],
      },
    };
    expect(said(named, "card", { key: "RAM-12", fields: { title: "Token refresh is flaky", assignee: "  ", body: "" } })).toEqual([["title", "title", "Token refresh is flaky"]]);
  });

  it("says a field by the block's label, then the one it was declared with, then its name", () => {
    const named: Named = {
      ...issue,
      fields: [...issue.fields!, { name: "sprint", type: "text" }],
      views: {
        card: [
          { block: "field", field: "assignee", label: "Whose" },
          { block: "field", field: "assignee" },
          { block: "field", field: "sprint" },
        ],
      },
    };
    expect(thingBlocks(named, { key: "RAM-12", fields: { assignee: "Dana", sprint: "4" } }, "card").map((d) => d.label)).toEqual(["Whose", "Assignee", "sprint"]);
  });

  it("says a time as a time, and draws a type ws doesn't know as text", () => {
    const named: Named = {
      ...issue,
      views: {
        card: [
          { block: "field", field: "updated" },
          { block: "subtitle", field: "updated" },
          { block: "field", field: "points" },
          { block: "markdown", field: "updated" },
        ],
      },
    };
    expect(thingBlocks(named, record, "card").map((d) => [d.block, d.time ?? false, d.label])).toEqual([
      ["field", true, "Updated"],
      ["subtitle", true, undefined],
      ["field", false, "Points"],
      ["markdown", false, undefined],
    ]);
  });

  it("puts a badge on the line of the title, subtitle or badge before it, and gives the first title the icon", () => {
    const named: Named = {
      ...issue,
      views: {
        card: [
          { block: "badge", field: "status" },
          { block: "title", field: "title" },
          { block: "badge", field: "status" },
          { block: "badge", field: "assignee" },
          { block: "field", field: "assignee" },
          { block: "badge", field: "status" },
          { block: "title", field: "title" },
          { block: "subtitle", field: "nothing" },
          { block: "badge", field: "status" },
        ],
      },
    };
    expect(thingBlocks(named, record, "card").map((d) => [d.block, d.inline ?? false, d.icon ?? false])).toEqual([
      ["badge", false, false],
      ["title", false, true],
      ["badge", true, false],
      ["badge", true, false],
      ["field", false, false],
      ["badge", false, false],
      ["title", false, false],
      ["badge", true, false], // the subtitle before it had nothing to show
    ]);
  });

  it("keeps markdown to the lines its block says, when it says", () => {
    const named: Named = { ...issue, views: { card: [{ block: "markdown", field: "body", lines: 0 }, { block: "markdown", field: "body", lines: 3.9 }] } };
    expect(thingBlocks(named, record, "card").map((d) => d.lines)).toEqual([undefined, 3]);
  });

  it("makes a view from the fields where the plugin declared none", () => {
    // The pane wasn't declared: the card's, then its text and time fields
    // (a type ws doesn't know among them), then its markdown.
    expect(said(issue, "pane")).toEqual([
      ["title", "title", "Token refresh is flaky"],
      ["subtitle", "key", "RAM-12"],
      ["badge", "status", "In Progress"],
      ["field", "assignee", "Dana"],
      ["field", "updated", "2026-11-18T10:42:00Z"],
      ["field", "points", "3"],
      ["markdown", "body", "The **clock** should be injected."],
    ]);
    const plain: Named = { ...issue, views: undefined };
    expect(said(plain, "card")).toEqual([
      ["title", "title", "Token refresh is flaky"],
      ["subtitle", "key", "RAM-12"],
      ["badge", "status", "In Progress"],
    ]);
    // A view declared with nothing in it is none declared.
    expect(viewOf({ ...issue, views: { card: [] } }, "card")).toHaveLength(3);
    // With no title or state among its fields, its name still says it.
    expect(said({ ...plain, fields: [{ name: "assignee", type: "text" }] }, "card")).toEqual([["subtitle", "key", "RAM-12"]]);
  });

  it("draws nothing of a kind with no fields, or one ws doesn't keep", () => {
    expect(thingBlocks({ plugin: "jira", what: "sprint", prefixes: ["SPRINT-"] }, record, "pane")).toEqual([]);
    expect(thingBlocks(undefined, record, "card")).toEqual([]);
  });

  it("reads a field that names other things as its names, each apart", () => {
    expect(namesIn("RAM-10")).toEqual(["RAM-10"]);
    expect(namesIn(" RAM-10, RAM-11\nRAM-12  api#7 ,")).toEqual(["RAM-10", "RAM-11", "RAM-12", "api#7"]);
    expect(namesIn("  ")).toEqual([]);

    const linked: Named = {
      ...issue,
      fields: [...(issue.fields ?? []), { name: "parent", type: "ref", label: "Parent" }, { name: "children", type: "refs", label: "Children" }],
      views: { card: [{ block: "title", field: "title" }, { block: "field", field: "parent" }, { block: "badge", field: "parent" }] },
    };
    const r = { key: "RAM-12", fields: { ...record.fields, parent: "RAM-10", children: "RAM-13, RAM-14 not-a-name" } };
    // In a field block, its names; in any other, its words.
    expect(thingBlocks(linked, r, "card").slice(1)).toEqual([
      { block: "field", field: "parent", value: "RAM-10", label: "Parent", names: ["RAM-10"] },
      { block: "badge", field: "parent", value: "RAM-10" },
    ]);
    // With no view of the plugin's, the pane has them with its other
    // fields, before what it says at length; the card doesn't.
    const plain = { ...linked, views: undefined };
    expect(viewOf(plain, "pane").map((b) => `${b.block}:${b.field}`)).toEqual(["title:title", "subtitle:key", "badge:status", "field:assignee", "field:updated", "field:points", "field:parent", "field:children", "markdown:body"]);
    expect(viewOf(plain, "card").map((b) => b.field)).toEqual(["title", "key", "status"]);
    expect(thingBlocks(plain, r, "pane").find((d) => d.field === "children")).toEqual({ block: "field", field: "children", value: "RAM-13, RAM-14 not-a-name", label: "Children", names: ["RAM-13", "RAM-14", "not-a-name"] });
  });
});
