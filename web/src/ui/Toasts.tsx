import { useSyncExternalStore } from "react";
import css from "./Toasts.module.css";
import { ToastPill } from "./ToastPill";
import { getToasts, onToasts, setToasts } from "./toast";

// Toasts draws what's being said in passing (ui/toast.ts): for a few
// seconds, bottom centre, over everything, an open dialog too. One with
// something to press goes once it's pressed.
export function Toasts() {
  const list = useSyncExternalStore(onToasts, getToasts);
  return (
    <div className={css.toasts} role="status" aria-live="polite" data-toasts="" data-react-aria-top-layer="">
      {list.map((t) => (
        <ToastPill
          key={t.id}
          tone={t.tone}
          action={
            t.action && {
              label: t.action.label,
              run: () => {
                t.action!.run();
                setToasts(getToasts().filter((x) => x.id !== t.id));
              },
            }
          }
        >
          {t.text}
        </ToastPill>
      ))}
    </div>
  );
}
