import { useEffect, useState } from "react";
import { type ActionRan, type AutomationAction, getActions, runAction } from "../api";
import { toast } from "../ui/toast";
import { humanError } from "../ui/strings";
import { askActionInput } from "./askActionInput";
import { confirmAct } from "./confirmAct";

// Your automations' actions (when = action(...)), where you act on things:
// those on nothing in particular under Find's Do, those on a thing in
// Find's Here on its page and in its row's menu. What's offered is the
// server's to say (a thing's fields, as they stand, choose them), asked
// once a thing a while; running one asks first what it asks, then says
// what it did, there and then.

// fresh is how long what's offered on a thing is taken as it is.
const fresh = 20_000;

const kept = new Map<string, { at: number; got: Promise<AutomationAction[]> }>();

// actionsOn is what's offered on on (a ref), or on nothing for "".
export function actionsOn(on: string): Promise<AutomationAction[]> {
  const had = kept.get(on);
  if (had && Date.now() - had.at < fresh) return had.got;
  const got = getActions(on || undefined).then(
    (r) => r.actions,
    () => [],
  );
  kept.set(on, { at: Date.now(), got });
  return got;
}

// forgetActions has what's offered asked again: an automation's changed.
export function forgetActions() {
  kept.clear();
}

// useActions is what's offered on on, once it's known: none while it's
// asked, and none for undefined (somewhere that isn't a thing).
export function useActions(on: string | undefined): AutomationAction[] {
  const [got, setGot] = useState<{ on: string; list: AutomationAction[] }>();
  useEffect(() => {
    if (on === undefined) return;
    let live = true;
    actionsOn(on).then((list) => live && setGot({ on, list }));
    return () => {
      live = false;
    };
  }, [on]);
  return on !== undefined && got?.on === on ? got.list : [];
}

// ranSaid is how a run went, in a toast's words: what it said or made,
// that it's waiting (on what), or why it couldn't. open is where what it
// made is, if it made something to go to.
export function ranSaid(a: AutomationAction, r: ActionRan): { text: string; tone: "info" | "danger" | "done"; open?: { label: string; to: string } } {
  const did = r.did ?? [];
  const made = did.find((d) => d.what === "propose" && d.conversation);
  const open = made ? { label: "Open", to: `/conversations/${encodeURIComponent(made.conversation ?? "")}` } : undefined;
  switch (r.state) {
    case "failed":
    case "unsure":
      return { text: `${a.title} couldn't run: ${r.error ?? "it failed"}`, tone: "danger" };
    case "held":
      return { text: `${a.title} was held: ${r.error ?? ""}`, tone: "info" };
    case "running":
      return { text: `${a.title} is running: its automation's page says how it goes.`, tone: "info" };
    case "waiting": {
      const on = did.filter((d) => d.waiting).pop();
      return { text: `${a.title} is waiting${on?.text ? ` ${on.text}` : ""}.`, tone: "info", open };
    }
  }
  const said = did.find((d) => d.what === "notify" && d.text)?.text;
  if (made) return { text: `${a.title}: proposed ${made.text ?? "something"} for you to accept.`, tone: "done", open };
  return { text: said ?? `${a.title}: done.`, tone: "done" };
}

// doAction runs a on on ("" for nothing): what it asks first is asked
// in a dialog (put away, it isn't run), then it's run, and what it did is
// said; go is how its Open goes somewhere.
export async function doAction(a: AutomationAction, on: string | undefined, go: (to: string) => void) {
  let input: Record<string, unknown> | undefined;
  if (a.input?.length) {
    input = await askActionInput(a, on);
    if (!input) return;
  }
  try {
    const r = await runAction({ automation: a.automation, title: a.title, on: on || undefined, input });
    // What it would do through a plugin, it does once you've said so:
    // its card, here, now.
    const acting = (r.did ?? []).find((d) => d.what === "act" && d.settled === "open" && d.proposal);
    if (acting?.proposal) {
      const [conversation, n] = acting.proposal.split("#");
      const said = await confirmAct(conversation, Number(n), a.title);
      if (said) toast(`${a.title}: ${said}`, { tone: "done" });
      return;
    }
    const said = ranSaid(a, r);
    toast(said.text, { tone: said.tone, action: said.open && { label: said.open.label, run: () => go(said.open!.to) } });
  } catch (e) {
    toast(humanError((e as Error).message), { tone: "danger" });
  }
}
