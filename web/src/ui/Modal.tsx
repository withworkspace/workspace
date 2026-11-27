import type { ReactNode, Ref } from "react";
import { Dialog as AriaDialog, Modal as AriaModal, ModalOverlay } from "react-aria-components";
import { useCommands } from "../keymap/useCommands";
import { quietKeys } from "./partKeys";
import { cx } from "./cx";
import css from "./Modal.module.css";

// Modal is what's drawn over a dimmed page, which can't be reached while
// it's open: a Dialog is one with a head and a look; a finder is one
// with a look of its own, which it gives as className. It's drawn while
// it's open (whoever opens it draws it then), and onClose is called by a
// press outside it, and by Escape unless it says escape={false}, where
// Escape is its own to hear (a step back, before closing).
//
// react-aria's modal does the rest: the focus is kept inside it and given
// back when it closes, the page behind is inert, and a screen reader
// hears a dialog named by aria-label. The page's keys are quiet while
// it's open (keys.ts). What's drawn over it from elsewhere on the page (a
// hover card, the toasts: data-react-aria-top-layer) isn't outside it: a
// press there doesn't close it.
//
// data-overlay says something's open, to whoever asks; overlayClassName
// is the dimmed page's, for one that places itself otherwise.
export function Modal({
  onClose,
  escape = true,
  className,
  overlayClassName,
  "aria-label": label,
  ref,
  children,
}: {
  onClose: () => void;
  escape?: boolean;
  className?: string;
  overlayClassName?: string;
  "aria-label"?: string;
  ref?: Ref<HTMLElement>;
  children: ReactNode;
}) {
  useCommands(quietKeys, {}, { modal: true });
  return (
    <ModalOverlay
      isOpen
      isDismissable
      isKeyboardDismissDisabled={!escape}
      shouldCloseOnInteractOutside={(el) => !el.closest("[data-react-aria-top-layer]")}
      onOpenChange={(open) => !open && onClose()}
      className={cx(css.overlay, overlayClassName)}
      data-overlay=""
    >
      <AriaModal className={css.modal}>
        <AriaDialog ref={ref} className={cx(css.box, className)} aria-label={label}>
          {children}
        </AriaDialog>
      </AriaModal>
    </ModalOverlay>
  );
}
