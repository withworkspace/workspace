import { describe, expect, it } from "vitest";
import { frameHere, setFrameHere } from "./here";

describe("what a plugin's page says it shows", () => {
  it("is the latest said, and none once its page says none or goes", () => {
    setFrameHere("jira", "RAM-12");
    expect(frameHere()).toEqual({ plugin: "jira", name: "RAM-12" });
    setFrameHere("jira", undefined);
    expect(frameHere()).toBeUndefined();
  });

  it("isn't forgotten by another plugin's page going as this one comes", () => {
    setFrameHere("jira", "RAM-12");
    setFrameHere("relman", undefined);
    expect(frameHere()).toEqual({ plugin: "jira", name: "RAM-12" });
    setFrameHere("jira", undefined);
  });
});
