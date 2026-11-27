import { useState, useSyncExternalStore } from "react";
import { Button } from "../ui/Button";
import { Dialog, DialogActions, DialogText } from "../ui/Dialog";
import { Text } from "../ui/Text";
import { ActionInputForm } from "./ActionInputForm";
import { answered, inputAsked, onInputAsked } from "./askActionInput";

// ActionInputAsker draws what askActionInput asks: once, by the app. It
// starts each field as its kind starts (a choice on its first, a yes
// unticked), and Run gives what's there.
export function ActionInputAsker() {
  const ask = useSyncExternalStore(onInputAsked, inputAsked);
  if (!ask) return null;
  return <Asking key={`${ask.action.automation}/${ask.action.title}/${ask.on ?? ""}`} />;
}

function Asking() {
  const ask = inputAsked();
  const fields = ask?.action.input ?? [];
  const [values, setValues] = useState<Record<string, unknown>>(() =>
    Object.fromEntries(fields.map((f) => [f.name, f.type === "bool" ? false : f.type === "choice" ? f.choices?.[0] : ""])),
  );
  if (!ask) return null;
  const go = () => answered(values);
  return (
    <Dialog title={ask.action.title} onClose={() => answered(undefined)}>
      <DialogText>
        {ask.on ? (
          <>
            On <Text mono>{ask.on}</Text>, by your automation {ask.action.automation}.
          </>
        ) : (
          <>By your automation {ask.action.automation}.</>
        )}
      </DialogText>
      <ActionInputForm fields={fields} values={values} onChange={(name, v) => setValues((was) => ({ ...was, [name]: v }))} onGo={go} />
      <DialogActions>
        <Button onClick={() => answered(undefined)} keys="esc">
          Cancel
        </Button>
        <Button variant="primary" onClick={go}>
          Run
        </Button>
      </DialogActions>
    </Dialog>
  );
}
