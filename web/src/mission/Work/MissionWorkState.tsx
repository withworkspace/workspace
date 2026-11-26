import type { MouseEvent } from "react";
import { chipClass } from "../../ui/Chip";
import { Icon, type IconName } from "../../ui/Icon";
import { keyProps } from "../../ui/keyHint";
import { LiveDot } from "../../ui/LiveDot";
import type { Tone } from "../../ui/tone";

// MissionWorkState is where a capsule stands, in a chip: crew at work in
// it (live), yours to check, landed. With its pull request's address
// (url) the chip goes there.
export function MissionWorkState({ text, tone, icon, live, url, keys }: { text: string; tone?: Tone; icon?: IconName; live?: boolean; url?: string; keys?: string }) {
  const body = (
    <>
      {live ? <LiveDot /> : icon && <Icon name={icon} size={11} />}
      {text}
    </>
  );
  const cls = chipClass(tone ?? "quiet");
  return url ? (
    <a href={url} target="_blank" rel="noopener noreferrer" className={cls} {...keyProps(keys, keys ? "Open pull request" : undefined, (e: MouseEvent) => e.stopPropagation())}>
      {body}
    </a>
  ) : (
    <span className={cls}>{body}</span>
  );
}
