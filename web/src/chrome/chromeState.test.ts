import { describe, expect, it } from "vitest";
import { hideSidebar, read, resting, sidebarPlace, write } from "./chromeState";

describe("where the sidebar is", () => {
  it("is the page's to say between open and the rail", () => {
    expect(sidebarPlace(resting, false)).toBe("open");
    expect(sidebarPlace(resting, true)).toBe("rail");
  });

  it("stays hidden once you hide it, whatever the page wants", () => {
    const hidden = hideSidebar(resting, true);
    expect(sidebarPlace(hidden, false)).toBe("hidden");
    expect(sidebarPlace(hidden, true)).toBe("hidden");
    expect(sidebarPlace(hideSidebar(hidden, false), true)).toBe("rail");
  });

  it("is a drawer on a narrow window, whatever the page wants or you chose", () => {
    expect(sidebarPlace(resting, false, true)).toBe("drawer");
    expect(sidebarPlace(resting, true, true)).toBe("drawer");
    expect(sidebarPlace(hideSidebar(resting, true), true, true)).toBe("drawer");
  });
});

describe("the arrangement, kept in the browser", () => {
  it("comes back as it was written", () => {
    const s = hideSidebar(resting, true);
    expect(read(write(s))).toEqual(s);
  });

  it("rests when nothing was kept, or what was kept isn't it", () => {
    expect(read(null)).toEqual(resting);
    expect(read("")).toEqual(resting);
    expect(read("not json")).toEqual(resting);
    expect(read("[1, 2]")).toEqual(resting);
    expect(read('"open"')).toEqual(resting);
    expect(read('{"hidden":"yes"}')).toEqual(resting);
  });

  it("takes no notice of what a later ws kept beside it", () => {
    expect(read('{"hidden":true,"beside":{"open":true}}')).toEqual({ hidden: true });
  });
});
