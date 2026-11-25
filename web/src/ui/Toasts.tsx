import { useRef } from "react";
import { UNSAFE_PortalProvider as PortalProvider } from "react-aria";
import { UNSTABLE_Toast as Toast, UNSTABLE_ToastRegion as ToastRegion } from "react-aria-components";
import { ToastPill } from "./ToastPill";
import css from "./Toasts.module.css";
import { type ToastItem, toasts } from "./toast";

// Toasts draws what's being said in passing (ui/toast.ts): for a few
// seconds, in the window's bottom right corner, over everything, an open
// dialog too, the newest at the bottom. It's react-aria's toast region:
// each is read out as it comes, kept while it's pointed at or focused,
// and closed with its ×; one with something to press goes once it's
// pressed.
//
// react-aria puts the region at the end of the page. It's put here
// instead, in the shell, whose foot (--foot-height) it stands clear of.
export function Toasts() {
  const home = useRef<HTMLDivElement>(null);
  return (
    <div ref={home} className={css.home}>
      <PortalProvider getContainer={() => home.current ?? document.body}>
        <ToastRegion<ToastItem> queue={toasts} className={css.toasts} data-toasts="">
          {({ toast: t }) => (
            <Toast toast={t} className={css.toast}>
              <ToastPill
                tone={t.content.tone}
                keys={t.content.keys}
                action={
                  t.content.action && {
                    label: t.content.action.label,
                    run: () => {
                      t.content.action!.run();
                      toasts.close(t.key);
                    },
                  }
                }
                closable
              >
                {t.content.text}
              </ToastPill>
            </Toast>
          )}
        </ToastRegion>
      </PortalProvider>
    </div>
  );
}
