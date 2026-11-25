import { describe, expect, it } from "vitest";
import { standingOf } from "./standing";

const up = (over: Partial<Parameters<typeof standingOf>[0]> = {}) => ({ version: "v1.3.0", release: true, newer: false, ...over });

describe("where ws stands against its latest release", () => {
  it("isn't known before a look's answered", () => {
    expect(standingOf(up())).toBe("unknown");
  });
  it("is the latest, or ahead of it", () => {
    expect(standingOf(up({ latest: "v1.3.0" }))).toBe("latest");
    expect(standingOf(up({ version: "v1.4.0-rc.1", latest: "v1.3.0" }))).toBe("ahead");
  });
  it("is behind: to upgrade here, where it's a release's build", () => {
    expect(standingOf(up({ latest: "v1.4.0", newer: true }))).toBe("out");
    expect(standingOf(up({ latest: "v1.4.0", newer: true, release: false }))).toBe("out-built");
  });
  it("is waiting, once the later one's installed", () => {
    expect(standingOf(up({ latest: "v1.4.0", newer: true, installed: "v1.4.0" }))).toBe("waiting");
  });
});
