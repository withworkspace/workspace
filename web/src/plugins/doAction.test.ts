import { describe, expect, it } from "vitest";
import { pluginTab } from "./doAction";

describe("a plugin's tab beside", () => {
  it("is the one of its app's, whatever path it's at", () => {
    const tabs = ["/missions/fix-it", "/x/jira/board", "/x/sketch/new"];
    expect(pluginTab(tabs, "/x/jira/new?about=fix-it")).toBe("/x/jira/board");
    expect(pluginTab(tabs, "/x/sketch")).toBe("/x/sketch/new");
    expect(pluginTab(tabs, "/x/kit/new")).toBeUndefined();
    expect(pluginTab(tabs, "/missions/other")).toBeUndefined();
  });
});
