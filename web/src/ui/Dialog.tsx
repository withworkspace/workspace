import { type ReactNode, useLayoutEffect, useRef } from "react";
import { Heading } from "react-aria-components";
import { Button } from "./Button";
import { cx } from "./cx";
import css from "./Dialog.module.css";
import { Icon } from "./Icon";
import { Modal } from "./Modal";

// Dialog is a Modal with a head: its title, and a button that closes it.
// It's drawn while it's open (whoever opens it draws it then), and
// onClose is called by Escape, a press outside it, or its close button.
//
// The focus starts on what says autoFocus, or else the first field to
// type in (not the close button, nor a Select).
//
// A wide one has room for fields. A bare one draws its own head: title is
// only its name, and the close button sits over its corner. In it,
// DialogText is what it says and DialogActions its buttons, at its foot.
export function Dialog({ title, onClose, children, className, wide, bare }: { title: ReactNode; onClose: () => void; children: ReactNode; className?: string; wide?: boolean; bare?: boolean }) {
  const ref = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || el.contains(document.activeElement)) return;
    el.querySelector<HTMLElement>(":is(textarea, input, button, [tabindex]:not([tabindex='-1'])):not([data-dialog-close], [data-select])")?.focus();
  }, []);
  const close = (
    <Button variant="ghost" square className={bare ? css.closeCorner : undefined} data-dialog-close="" onClick={onClose} title="Close (esc)" aria-label="Close">
      <Icon name="x" size={14} />
    </Button>
  );
  return (
    <Modal ref={ref} onClose={onClose} className={cx(css.dialog, wide && css.wide, className)} aria-label={typeof title === "string" ? title : undefined}>
      {bare ? (
        close
      ) : (
        <header className={css.head}>
          <Heading slot="title" level={2} className={css.title}>
            {title}
          </Heading>
          {close}
        </header>
      )}
      {children}
    </Modal>
  );
}

// DialogText is what a dialog says, quietly, over its fields or buttons.
export function DialogText({ children }: { children: ReactNode }) {
  return <div className={css.text}>{children}</div>;
}

// DialogActions is a dialog's buttons, at its foot's end: Cancel, then
// the one that does it.
export function DialogActions({ children }: { children: ReactNode }) {
  return <div className={css.actions}>{children}</div>;
}
