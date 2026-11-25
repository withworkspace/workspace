import type { ReactNode } from "react";
import css from "./ThingUi.module.css";

// ThingUi is what one of ws's UIs for a plugin's thing says, in its panel
// (OverFrame): its lines in a column. Given onSubmit, it's a form, which
// ⏎ in a field of it sends.
export function ThingUi({ onSubmit, role, children }: { onSubmit?: () => void; role?: string; children: ReactNode }) {
  if (onSubmit) {
    return (
      <form
        className={css.ui}
        role={role}
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit();
        }}
      >
        {children}
      </form>
    );
  }
  return (
    <div className={css.ui} role={role}>
      {children}
    </div>
  );
}

// ThingUiTitle is what it asks, or what didn't work, at its head.
export function ThingUiTitle({ children }: { children: ReactNode }) {
  return <strong className={css.title}>{children}</strong>;
}

// ThingUiFailed is why it didn't work.
export function ThingUiFailed({ children }: { children: ReactNode }) {
  return (
    <p className={css.failed} role="alert">
      {children}
    </p>
  );
}

// ThingUiButtons is what's pressed, at its foot.
export function ThingUiButtons({ children }: { children: ReactNode }) {
  return <div className={css.buttons}>{children}</div>;
}
