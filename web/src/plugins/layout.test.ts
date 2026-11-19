import { describe, expect, it } from "vitest";
import { frameExpanded, setFrameLayout } from "./layout";

describe("a plugin's page that asks for the chrome expanded", () => {
  it("has it until its page says standard, or goes", () => {
    setFrameLayout("sketch", "expanded");
    expect(frameExpanded()).toBe("sketch");
    setFrameLayout("sketch", "standard");
    expect(frameExpanded()).toBeUndefined();
  });

  it("keeps it as another plugin's page goes", () => {
    setFrameLayout("sketch", "expanded");
    setFrameLayout("jira", "standard");
    expect(frameExpanded()).toBe("sketch");
    setFrameLayout("sketch", "standard");
  });
});
