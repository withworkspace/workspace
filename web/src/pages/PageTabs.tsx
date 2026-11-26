import type { ReactNode } from "react";
import { Tabs } from "../ui/Tabs";
import css from "./PageTabs.module.css";

// PageTabs are the page's tab and a tab for each pass over it.
export function PageTabs({ value, onChange, children }: { value: string; onChange: (v: string) => void; children: ReactNode }) {
  return (
    <Tabs className={css.tabs} value={value} onChange={onChange} aria-label="Page and passes">
      {children}
    </Tabs>
  );
}
