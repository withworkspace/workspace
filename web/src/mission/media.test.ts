import { describe, expect, it } from "vitest";
import { mediaKind } from "./media";

describe("mediaKind", () => {
  it.each([
    ["settled-markers-dark.png", "image"],
    ["mission:shots/Page.JPG", "image"],
    ["diagram.svg", "image"],
    ["run.webm", "video"],
    ["walkthrough.MP4", "video"],
    ["handoffs/c1.md", undefined],
    ["png-notes.txt", undefined],
  ])("%s is %s", (name, kind) => expect(mediaKind(name)).toBe(kind));
});
