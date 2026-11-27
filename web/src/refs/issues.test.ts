import { describe, expect, it } from "vitest";
import { ApiError, type PR } from "../api";
import { isFrom } from "./issues";

// What the pull request's card is answered, as what an issue's number is.
// (Not by mocking prs.ts: test files share their modules, vite.config.ts.)
describe("what an issue's number is, as GitHub says", () => {
  it("is the pull request, where it's an open one", async () => {
    const pr = { title: "Limit each key" } as PR;
    expect(await isFrom(Promise.resolve(pr))).toEqual({ is: "pr", pr });
  });
  it("is the issue, where it isn't an open pull request", async () => {
    expect(await isFrom(Promise.reject(new ApiError(404, undefined, "not open")))).toEqual({ is: "issue" });
  });
  it("is unknown, where GitHub couldn't be asked", async () => {
    expect(await isFrom(Promise.reject(new ApiError(422, undefined, "gh: not signed in")))).toEqual({ is: "unknown" });
    expect(await isFrom(Promise.reject(new TypeError("Failed to fetch")))).toEqual({ is: "unknown" });
  });
});
