import { describe, expect, test } from "vitest";
import { pictureURL } from "./pictures";

describe("pictureURL", () => {
  const name = "0123456789abcdef0123456789abcdef.png";
  test("a pasted picture's path is drawn from ws", () => {
    expect(pictureURL(`/Users/me/my work/.ws/images/${name}`)).toBe(`/api/images/${name}`);
    expect(pictureURL(`C:\\work\\.ws\\images\\${name}`)).toBe(`/api/images/${name}`);
  });
  test("anything else isn't one", () => {
    expect(pictureURL("docs/shot.png")).toBeUndefined();
    expect(pictureURL(`/work/.ws/images/${name}/../ws.db`)).toBeUndefined();
    expect(pictureURL(`https://example.com/.ws/images/x.png`)).toBeUndefined();
    expect(pictureURL(undefined)).toBeUndefined();
  });
});
