import type { IconName } from "./Icon";
import { Status } from "./Status";
import type { Tone } from "./tone";

// StatusIcon is an icon in a tone, a status's look with no words: it
// stands for a kind of thing (a pin that's a conversation, a pull
// request) or leads what says the rest (a mission's lead). live pulses
// it; a title is its tooltip, where the tone alone wouldn't say why.
export function StatusIcon({ icon, tone, live, size, title, className }: { icon: IconName; tone: Tone; live?: boolean; size?: number; title?: string; className?: string }) {
  return <Status s={{ icon, tone, words: "", live }} iconOnly size={size} title={title} className={className} />;
}
