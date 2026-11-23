import { describe, expect, it } from "vitest";
import { type BelowView, belowView } from "./Below";

const view = (id: string): BelowView => ({ id, name: id, icon: "play", pane: () => null });

describe("the view that's showing below", () => {
  const views = [view("running"), view("results")];

  it("is the one last picked", () => {
    expect(belowView(views, "results")?.id).toBe("results");
  });

  it("is the first where none was picked, or the one picked has gone", () => {
    expect(belowView(views)?.id).toBe("running");
    expect(belowView(views, "run:dev")?.id).toBe("running");
  });

  it("is none where there are no views", () => {
    expect(belowView([], "results")).toBeUndefined();
  });
});
