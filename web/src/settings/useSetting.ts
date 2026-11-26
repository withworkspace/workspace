import type { Setting, SettingChange } from "../api";
import { usePage } from "./page";
import { valueSaid } from "./sections";

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);
export const has = (v: unknown) => v !== undefined && v !== null;

// back is the change that puts yours back as it is now.
const back = (x: Setting): SettingChange => (has(x.yours) ? { section: x.section, key: x.key, value: x.yours } : { section: x.section, key: x.key, clear: true });

// without is what a setting goes back to, reset.
export const without = (s: Setting) =>
  has(s.team) ? `the team's, ${valueSaid(s, s.team) || "none"}` : `the default, ${valueSaid(s, s.default) || (s.kind === "picks" ? "none" : s.kind === "color" ? "Workspace's own" : s.kind === "places" ? "as Workspace has them" : "the agent's own")}`;

// useSetting is a setting's two writes: set (a value of yours) and reset
// (yours taken away), each undone from its toast.
export function useSetting(s: Setting): { set: (v: unknown) => Promise<void>; reset: () => void } {
  const { all, change } = usePage();
  const at = { section: s.section, key: s.key };
  const set = (value: unknown) => {
    if (same(value, s.value) && s.source === "yours") return Promise.resolve();
    const changes: SettingChange[] = [{ ...at, value }];
    const undo = [back(s)];
    // A model belongs to the harness it was set for: changing a role's
    // harness takes your model for it away.
    const model = all.find((x) => x.section === s.section && x.kind === "model");
    if (s.kind === "harness" && model && has(model.yours)) {
      changes.push({ section: model.section, key: model.key, clear: true });
      undo.push(back(model));
    }
    return change(changes, undo);
  };
  // The places' two keys are one arrangement: reset together.
  const together = s.kind === "places" ? all.filter((x) => x.kind === "places" && has(x.yours)) : [s];
  const reset = () => void change(together.map((x) => ({ section: x.section, key: x.key, clear: true })), together.map(back));
  return { set, reset };
}
