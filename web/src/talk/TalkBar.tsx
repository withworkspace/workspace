import type { ComponentProps } from "react";
import { PageBar } from "../ui/PageBar";
import css from "./TalkBar.module.css";

// TalkBar is a conversation's bar: a PageBar that spans the page, beside
// the column of what's come of it too, and over its padding.
export function TalkBar(props: Omit<ComponentProps<typeof PageBar>, "className">) {
  return <PageBar {...props} className={css.bar} />;
}
