import { useEffect, useState } from "react";
import type { ReviewGuide } from "../api";
import { Spinner } from "../ui/Spinner";
import { duration } from "../ui/when";
import { leftWords } from "./guideWords";
import css from "./GuideWriting.module.css";
import { Text } from "../ui/Text";

// GuideWriting says the guide's being written (again): for how long, on a
// clock of its own that ticks each second, whether its agent's thinking it
// over, and what it was given to write it from.
export function GuideWriting({ since, thinking, given, again }: { since?: string; thinking?: boolean; given?: ReviewGuide["given"]; again: boolean }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!since) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [since]);
  const left = given?.left ?? [];
  return (
    <div className={css.writing}>
      <Text as="p" dim>
        <Spinner size={12} /> {!since ? "Waiting to start…" : again ? "Writing it again" : "Writing the guide"}
        {since && <span title="Since it started"> · {duration(since, now)}</span>}
        {thinking && <span title="Its agent thinks before it writes: what it thinks isn't shown"> · thinking it over</span>}
      </Text>
      {given && (
        <Text as="p" dim className={css.given} title={left.length ? `Not shown to it:\n${left.map((l) => `${l.path} (${l.why})`).join("\n")}` : undefined}>
          From the description and the changes to {given.files.length - left.length} of {given.files.length} {given.files.length === 1 ? "file" : "files"}
          {left.length ? ` · not shown: ${leftWords(left)}` : ""}
        </Text>
      )}
    </div>
  );
}
