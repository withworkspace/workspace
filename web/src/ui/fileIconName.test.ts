import { describe, expect, it } from "vitest";
import { fileIconName } from "./fileIconName";
import { fileIcons, iconByExt, iconByName } from "./fileIcons.gen";

describe("fileIconName", () => {
  it("names a file's language by its extension", () => {
    expect(fileIconName("internal/web/server.go")).toBe("go");
    expect(fileIconName("web/src/App.tsx")).toBe("react-ts");
    expect(fileIconName("scripts/x.PY")).toBe("python");
  });
  it("takes the longest extension it knows", () => {
    expect(fileIconName("src/ui/fuzzy.test.ts")).toBe("test-ts");
    expect(fileIconName("src/env.d.ts")).toBe("typescript-def");
    expect(fileIconName("a.b.c.json")).toBe("json");
  });
  it("takes a whole name over its extension", () => {
    expect(fileIconName("go.mod")).toBe("go-mod");
    expect(fileIconName("web/package.json")).toBe("nodejs");
    expect(fileIconName("Justfile")).toBe("just");
    expect(fileIconName("README.md")).toBe("readme");
    expect(fileIconName(".gitignore")).toBe("git");
  });
  it("gives a dotfile's name no extension", () => {
    expect(fileIconName(".zshrc")).toBeUndefined();
    expect(fileIconName(".eslintrc.json")).toBe("json");
  });
  it("names nothing for what it doesn't know", () => {
    expect(fileIconName("LICENSE.weird")).toBeUndefined();
    expect(fileIconName("Procfile")).toBeUndefined();
  });
  it("has every icon it names", () => {
    for (const n of [...Object.values(iconByName), ...Object.values(iconByExt)]) expect(fileIcons[n], n).toBeDefined();
  });
});
