import { describe, expect, it } from "vitest";
import { appliedRunLog, logLines } from "./runlog";

const held = { ref: "web.feat.demo", log: "starting\n", size: 9 };

describe("appliedRunLog", () => {
  it("follows on with what's written, a line not yet ended too", () => {
    const a = appliedRunLog(held, { from: 9, size: 25, text: "listening on :30" });
    expect(a).toEqual({ ref: held.ref, log: "starting\nlistening on :30", size: 25 });
    const b = appliedRunLog(a as typeof held, { from: 25, size: 28, text: "00\n" });
    expect((b as typeof held).log).toBe("starting\nlistening on :3000\n");
  });

  it("drops what it has, and reads it whole when it's from elsewhere", () => {
    expect(appliedRunLog(held, { from: 0, size: 9, text: "starting\n" })).toBeUndefined();
    expect(appliedRunLog(held, { from: 20, size: 30, text: "x" })).toBe("read");
    expect(appliedRunLog(undefined, { from: 0, size: 1, text: "x" })).toBe("read");
  });

  it("keeps the last lines it shows", () => {
    const many = { ref: held.ref, log: Array.from({ length: logLines }, (_, i) => `${i}`).join("\n") + "\n", size: 5000 };
    const got = appliedRunLog(many, { from: 5000, size: 5004, text: "new\n" }) as typeof held;
    const lines = got.log.split("\n");
    expect(lines).toHaveLength(logLines + 1); // and the "" after the last newline
    expect(lines[logLines - 1]).toBe("new");
    expect(lines[0]).toBe("1");
  });
});
