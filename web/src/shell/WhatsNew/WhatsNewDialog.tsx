import type { ReactNode } from "react";
import { Button } from "../../ui/Button";
import { Dialog } from "../../ui/Dialog";
import { Logotype } from "../../ui/Logotype";
import css from "./WhatsNewDialog.module.css";
import { WhatsNewVersion } from "./WhatsNewVersion";
import { Text, textClass } from "../../ui/Text";

type WhatsNewDialogProps = {
  title: string;
  // Which ws this is, as the menu says it.
  version: string;
  // The line under the title: since when, and how many things.
  lede: string;
  // The releases page, where some of what's new is from further back than
  // this ws keeps.
  releases?: string;
  onClose: () => void;
  // What's new: entries, or what's said where there are none.
  children: ReactNode;
};

// WhatsNewDialog is What's new: wide, with the horizon behind its head,
// what's new under it, and Got it at its foot.
export function WhatsNewDialog({ title, version, lede, releases, onClose, children }: WhatsNewDialogProps) {
  return (
    <Dialog title={title} onClose={onClose} bare className={css.dialog}>
      <header className={css.head}>
        <div className={css.eyebrow}>
          <Logotype height={22} className={css.logotype} />
          <WhatsNewVersion>{version}</WhatsNewVersion>
        </div>
        <h2>{title}</h2>
        <Text as="p" dim>{lede}</Text>
      </header>
      <div className={css.body}>{children}</div>
      <footer className={css.foot}>
        <Text dim>
          {releases && (
            <>
              For older changes, see{" "}
              <a className={textClass("link")} href={releases} target="_blank" rel="noopener noreferrer">
                the releases page
              </a>
              .
            </>
          )}
        </Text>
        <Button variant="primary" onClick={onClose} autoFocus>
          Got it
        </Button>
      </footer>
    </Dialog>
  );
}
