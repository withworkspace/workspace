import { CrewBadge } from "../mission/CrewBadge";
import { TurnAvatar, TurnHead } from "../ui/Turn";
import { whoName } from "./words";

// Who is whoever said a turn, over it: you, the agent, ws, or crew by
// their badge.
export function Who({ by, at }: { by: string; at?: string }) {
  const known = by === "you" || by === "agent" || by === "ws";
  return <TurnHead avatar={known ? <TurnAvatar by={by} /> : <CrewBadge id={by} />} who={whoName(by)} at={at} />;
}
