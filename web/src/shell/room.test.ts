import { describe, expect, it } from "vitest";
import { wantsRoom } from "./room";

describe("wantsRoom", () => {
  it("is so for a space of documents, not Documents or a document in it", () => {
    expect(wantsRoom("/documents/records")).toBe(true);
    expect(wantsRoom("/documents/records/")).toBe(true);
    expect(wantsRoom("/documents")).toBe(false);
    expect(wantsRoom("/documents/records/docs/verify-v2.md")).toBe(false);
    expect(wantsRoom("/documents/local/p1")).toBe(false);
  });
  it("is still so for a mission, a review, files and an artifact", () => {
    expect(wantsRoom("/missions/x")).toBe(true);
    expect(wantsRoom("/reviews/demo/4")).toBe(true);
    expect(wantsRoom("/files/demo")).toBe(true);
    expect(wantsRoom("/artifacts/a1")).toBe(true);
    expect(wantsRoom("/conversations")).toBe(false);
  });
});
