import type { ReactNode } from "react";
import { useTitle } from "../ui/useTitle";
import { WindowNotice } from "./WindowNotice";

// Notice is the window's page when there's nothing else to show, titled
// as it says: a WindowNotice.
export function Notice({ title, children }: { title: string; children: ReactNode }) {
  useTitle(title);
  return <WindowNotice title={title}>{children}</WindowNotice>;
}
