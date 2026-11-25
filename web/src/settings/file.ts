import type { SettingChange } from "../api";
import { name } from "./sections";

// fileName is a settings file's name, out of its path.
export const fileName = (path: string) => path.split("/").pop() ?? path;

// written is a change as it reads in the file, for its toast.
export function written(c: SettingChange): string {
  if (c.clear) return `${name(c)} cleared`;
  const quote = (x: unknown) => (typeof x === "string" ? `"${x}"` : String(x));
  const v = Array.isArray(c.value) ? `[${c.value.map(quote).join(", ")}]` : quote(c.value);
  return `${name(c)} = ${v}`;
}
