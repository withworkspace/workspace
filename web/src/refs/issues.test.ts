import { describe, expect, it, vi } from "vitest";
import { ApiError, type PR } from "../api";
import { issueIs } from "./issues";
import { prOf } from "./prs";

vi.mock("./prs", () => ({ prOf: vi.fn() }));
const asked = vi.mocked(prOf);

describe("what an issue's number is, as GitHub says", () => {
  it("is the pull request, where it's an open one", async () => {
    const pr = { title: "Limit each key" } as PR;
    asked.mockResolvedValueOnce(pr);
    expect(await issueIs("api", 38)).toEqual({ is: "pr", pr });
  });
  it("is the issue, where it isn't an open pull request", async () => {
    asked.mockRejectedValueOnce(new ApiError(404, undefined, "not open"));
    expect(await issueIs("api", 38)).toEqual({ is: "issue" });
  });
  it("is unknown, where GitHub couldn't be asked", async () => {
    asked.mockRejectedValueOnce(new ApiError(422, undefined, "gh: not signed in"));
    expect(await issueIs("api", 38)).toEqual({ is: "unknown" });
    asked.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    expect(await issueIs("api", 38)).toEqual({ is: "unknown" });
  });
});
