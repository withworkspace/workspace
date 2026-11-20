import { iconByExt, iconByName } from "./fileIcons.gen";

// fileIconName is the icon a file's path gets, or undefined: its whole
// name first, then its longest extension ("x.test.ts" before "x.ts").
export function fileIconName(path: string): string | undefined {
  const name = path.slice(path.lastIndexOf("/") + 1).toLowerCase();
  const byName = iconByName[name];
  if (byName) return byName;
  for (let dot = name.indexOf("."); dot >= 0; dot = name.indexOf(".", dot + 1)) {
    const ext = iconByExt[name.slice(dot + 1)];
    if (ext && dot > 0) return ext;
  }
  return undefined;
}
