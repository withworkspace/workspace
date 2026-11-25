import type { ReactNode } from "react";
import { Link } from "react-router";
import { Icon } from "../ui/Icon";
import css from "./PageHeader.module.css";
import { Text } from "../ui/Text";

// PageHeader is the bar over a page: back to where it's read (Documents,
// or the document), its name, then what's done with it (children).
export function PageHeader({ back, id, children }: { back: { to: string; label: string }; id: string; children?: ReactNode }) {
  return (
    <header className={css.header}>
      <Link to={back.to} className={css.back}>
        <Icon name="arrow-left" size={13} /> {back.label}
      </Link>
      <Text dim>{id}</Text>
      {children}
    </header>
  );
}
