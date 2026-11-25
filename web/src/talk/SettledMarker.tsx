import type { TalkTurn } from "../api";
import type { Settled } from "./proposals";
import { TalkSettled } from "./TalkSettled";

// SettledMarker is a turn ws wrote for you settling a card, which the
// agent read as yours, drawn as a TalkSettled: its title is the whole of
// the turn's first line; compact leaves out its time (in an ask's card).
export function SettledMarker({ m, t, compact }: { m: Settled; t: TalkTurn; compact?: boolean }) {
  return <TalkSettled accepted={m.accepted} what={m.what} to={m.to} edits={m.edits} said={t.body.split("\n")[0]} at={compact ? undefined : t.at} />;
}
