import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

// The cascade's layers are ordered by whoever names one first. The build
// splits the styles into files and may load a part's module (@layer ui)
// before styles.css, so index.html says the order before any of them.

const read = (file: string) => readFileSync(new URL(file, import.meta.url), "utf8");

describe("the order of the cascade's layers", () => {
  const order = /@layer ([a-z, ]+);/;

  it("is said in index.html, ahead of every stylesheet and script", () => {
    const html = read("../index.html");
    const at = html.search(order);
    expect(at).toBeGreaterThan(0);
    expect(at).toBeLessThan(html.search(/<script|rel="stylesheet"/));
  });

  it("is the same there as in styles.css", () => {
    expect(order.exec(read("../index.html"))?.[1]).toBe(order.exec(read("./styles.css"))?.[1]);
  });
});
