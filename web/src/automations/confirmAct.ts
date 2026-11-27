// confirmAct puts what an action you started would do through a plugin
// in front of you (ActConfirmer draws it): the proposal it made, its
// card, to accept or decline where you started it. It settles with what
// was said of it: undefined if it was put away, undecided.

export interface ActAsk {
  conversation: string;
  n: number;
  title: string;
  resolve: (said: string | undefined) => void;
}

let asking: ActAsk | undefined;
const listeners = new Set<() => void>();

export function confirmAct(conversation: string, n: number, title: string): Promise<string | undefined> {
  return new Promise((resolve) => {
    asking?.resolve(undefined);
    asking = { conversation, n, title, resolve };
    listeners.forEach((l) => l());
  });
}

export const actAsked = () => asking;
export function actAnswered(said: string | undefined) {
  const ask = asking;
  asking = undefined;
  listeners.forEach((l) => l());
  ask?.resolve(said);
}
export function onActAsked(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}
