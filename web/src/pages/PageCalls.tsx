import type { ComponentProps } from "react";
import { Calls } from "../ui/Calls";
import css from "./PageCalls.module.css";

// PageCalls are the steps an agent took finding something out, under a
// finding: Calls, a little apart from what's above them.
export function PageCalls(props: Omit<ComponentProps<typeof Calls>, "className">) {
  return <Calls className={css.calls} {...props} />;
}
