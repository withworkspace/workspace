import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import css from "./WindowUpdated.module.css";

// WindowUpdated says, across the top of the page, that ws was updated to
// another version (version, as it's shown) while the page was open, with
// Reload (onReload).
export function WindowUpdated({ version, onReload }: { version: string; onReload: () => void }) {
  return (
    <Banner
      tone="info"
      icon="rotate"
      role="status"
      className={css.updated}
      actions={
        <Button size="small" onClick={onReload}>
          Reload
        </Button>
      }
    >
      ws was updated to {version} while this page was open: reload it to use the new one.
    </Banner>
  );
}
