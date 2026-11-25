import { createContext, useContext } from "react";
import type { Setting, SettingChange } from "../api";
import type { useNotifications } from "../shell/notify";

// What the Settings page's rows are given by the page: every setting (a
// change to one can take another with it), the way to write, what Find
// has typed (marked in each row), and what the page's own rows do.
export interface Page {
  all: Setting[];
  change: (changes: SettingChange[], undo?: SettingChange[]) => Promise<void>;
  marked: string[];
  notify: Pick<ReturnType<typeof useNotifications>, "supported" | "on" | "toggle">;
}

export const PageContext = createContext<Page | null>(null);

export function usePage(): Page {
  const page = useContext(PageContext);
  if (!page) throw new Error("a settings row outside the Settings page");
  return page;
}
