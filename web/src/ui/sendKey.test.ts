import { describe, expect, it } from "vitest";
import { sendKeys, sent } from "./sendKey";

// What a key press does in a box you write in, by the key you've chosen
// to send with.
const press = (held: Partial<{ key: string; shiftKey: boolean; metaKey: boolean; ctrlKey: boolean }> = {}) => ({ key: "Enter", shiftKey: false, metaKey: false, ctrlKey: false, ...held });

describe("sent", () => {
  it("sends with ⌘⏎ or ctrl ⏎, and leaves ⏎ a new line, where that's chosen", () => {
    expect(sent(press(), "mod+enter")).toBe(false);
    expect(sent(press({ shiftKey: true }), "mod+enter")).toBe(false);
    expect(sent(press({ metaKey: true }), "mod+enter")).toEqual({ other: false });
    expect(sent(press({ ctrlKey: true }), "mod+enter")).toEqual({ other: false });
  });

  it("sends with ⏎, and leaves ⇧⏎ a new line, where that's chosen", () => {
    expect(sent(press(), "enter")).toEqual({ other: false });
    expect(sent(press({ shiftKey: true }), "enter")).toBe(false);
    expect(sent(press({ shiftKey: true, metaKey: true }), "enter")).toBe(false);
  });

  it("sends the other way with one more key held", () => {
    expect(sent(press({ metaKey: true, shiftKey: true }), "mod+enter")).toEqual({ other: true });
    expect(sent(press({ metaKey: true }), "enter")).toEqual({ other: true });
    expect(sent(press({ ctrlKey: true }), "enter")).toEqual({ other: true });
  });

  it("is no other key", () => {
    expect(sent(press({ key: "a", metaKey: true }), "mod+enter")).toBe(false);
    expect(sent(press({ key: "a" }), "enter")).toBe(false);
  });
});

describe("sendKeys", () => {
  it("says the keys as Keys reads them", () => {
    expect(sendKeys("mod+enter")).toBe("mod+⏎");
    expect(sendKeys("mod+enter", true)).toBe("mod+shift+⏎");
    expect(sendKeys("enter")).toBe("⏎");
    expect(sendKeys("enter", true)).toBe("mod+⏎");
  });
});
