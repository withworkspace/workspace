import { useEffect, useRef, useState } from "react";
import * as api from "../../api";
import { OverFrame } from "./OverFrame";
import { ThingSave } from "./ThingSave";
import { asked, said } from "./things";
import type { UiProps } from "./uiProps";

// How long what came of a save stays by the button before it goes.
const shown = 1400;

// ThingSaveUi saves one of a plugin's own things to the repo it lives in:
// asking for it is pressing save, so it saves at once, says by the button
// how it's going and what came of it, and goes. It stays only when it
// couldn't, with why.
export function ThingSaveUi({ asked: asking, plugin, anchor, done }: UiProps) {
  const thing = asked(asking.with);
  const [state, setState] = useState<{ at: "saving" } | { at: "saved"; did: api.ThingSaved } | { at: "failed"; why: string }>({ at: "saving" });
  const [again, setAgain] = useState(0);
  // Asked once for each try: a render made twice (StrictMode) saves once.
  const tried = useRef(-1);
  const finish = useRef(done);
  finish.current = done;
  // biome-ignore lint/correctness/useExhaustiveDependencies: again is the asking
  useEffect(() => {
    if (tried.current === again) return;
    tried.current = again;
    setState({ at: "saving" });
    let timer: ReturnType<typeof setTimeout> | undefined;
    api.saveThing(plugin, { kind: thing.kind, ref: thing.ref }).then(
      (did) => {
        setState({ at: "saved", did });
        timer = setTimeout(() => finish.current(did), shown);
      },
      (err) => setState({ at: "failed", why: said(err) }),
    );
    return () => clearTimeout(timer);
  }, [plugin, thing.kind, thing.ref, again]);

  const what = thing.title || thing.ref;
  const did = state.at === "saved" ? state.did : undefined;
  return (
    <OverFrame anchor={anchor} label={`Saving ${what}`} busy={state.at === "saving"} dismiss={() => done(did)}>
      <ThingSave
        what={what}
        saved={did && { repo: did.thing.repo, path: did.thing.path, changed: did.changed, merged: did.merged, capsule: did.capsule }}
        why={state.at === "failed" ? state.why : undefined}
        onAgain={() => setAgain((n) => n + 1)}
        onClose={() => done()}
      />
    </OverFrame>
  );
}
