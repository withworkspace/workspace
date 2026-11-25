import type { ReactNode } from "react";
import { Printed } from "../ui/Printed";
import css from "./AppWaiting.module.css";

// AppWaiting is what an app's card says while there's nothing to frame:
// the run coming up, or why there isn't one, with its last words (log).
export function AppWaiting({ children, log }: { children: ReactNode; log?: string }) {
  return (
    <div className={css.waiting}>
      {children}
      {log && <Printed className={css.log}>{log}</Printed>}
    </div>
  );
}
