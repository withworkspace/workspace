import { type ReactNode, useSyncExternalStore } from "react";
import { Button } from "./Button";
import { Dialog, DialogActions, DialogText } from "./Dialog";

// confirm asks before something that can't be taken back, in a dialog of
// the page's own (not the browser's).
interface ConfirmAsk {
  title: string;
  body?: ReactNode;
  confirm: string;
  danger?: boolean;
  resolve: (yes: boolean) => void;
}

let asking: ConfirmAsk | undefined;
const askListeners = new Set<() => void>();

export function confirm(ask: Omit<ConfirmAsk, "resolve">): Promise<boolean> {
  return new Promise((resolve) => {
    asking = { ...ask, resolve };
    askListeners.forEach((l) => l());
  });
}

// Confirmer draws what confirm asks: once, by the app. The focus starts on
// the button that does it, so Enter does, unless it's a danger: then on
// Cancel, and doing it takes a press of its own.
export function Confirmer() {
  const ask = useSyncExternalStore(
    (l) => {
      askListeners.add(l);
      return () => askListeners.delete(l);
    },
    () => asking,
  );
  if (!ask) return null;
  const done = (yes: boolean) => {
    asking = undefined;
    askListeners.forEach((l) => l());
    ask.resolve(yes);
  };
  return (
    <Dialog title={ask.title} onClose={() => done(false)}>
      {ask.body && <DialogText>{ask.body}</DialogText>}
      <DialogActions>
        <Button onClick={() => done(false)}>Cancel</Button>
        <Button variant={ask.danger ? "danger" : "primary"} onClick={() => done(true)} autoFocus={!ask.danger}>
          {ask.confirm}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
