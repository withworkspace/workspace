import { Link } from "react-router";
import { buttonClass } from "../ui/Button";
import { Empty } from "../ui/Empty";
import css from "./WindowNotFound.module.css";

// WindowNotFound is the page at an address that's nothing: it doesn't
// exist, or what it showed has gone. Back to missions.
export function WindowNotFound() {
  return (
    <div className={css.page}>
      <Empty icon="search" title="There's nothing here" action={<Link to="/" className={buttonClass()}>Back to missions</Link>}>
        This page doesn't exist, or what it showed has gone.
      </Empty>
    </div>
  );
}
