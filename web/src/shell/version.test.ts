import { describe, expect, it } from "vitest";
import { shortVersion } from "./version";

describe("ws's version, as the menu says it", () => {
  it("is a release's as it is", () => {
    expect(shortVersion("v0.1.0")).toBe("v0.1.0");
    expect(shortVersion("v1.4.0-rc.1")).toBe("v1.4.0-rc.1");
  });
  it("is the release a build is after, that it's a dev build, and its commit", () => {
    expect(shortVersion("v0.1.1-0.20261117213809-56368bcb84bf+dirty")).toBe("v0.1.1-dev · 56368bc");
    expect(shortVersion("v0.0.0-20261117213809-56368bcb84bf")).toBe("v0.0.0-dev · 56368bc");
    expect(shortVersion("v1.4.0-rc.1.0.20261117213809-56368bcb84bf")).toBe("v1.4.0-dev · 56368bc");
  });
  it("says a build with no version is one", () => {
    expect(shortVersion("dev")).toBe("dev build");
    expect(shortVersion("(devel)")).toBe("dev build");
  });
});
