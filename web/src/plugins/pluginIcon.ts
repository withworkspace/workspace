import { type IconName, icons } from "../ui/Icon";

// pluginIcon is the icon a plugin asked for, if ws has it.
export function pluginIcon(name?: string): IconName {
  return name && name in icons ? (name as IconName) : "box";
}
