import { useCallback, useEffect, useRef, useState } from "react";
import { getTranscript, type Transcript, useVisible } from "../../api";
import { useLatest } from "../../load";
import { pageSocket } from "../../socket";
import { appliedTranscript } from "../transcript";

// useTranscript follows a crew's transcript over the tab's socket while the
// page is showing (transcript:<slug>/<crew>): read whole once it's
// followed, then told each change, which is applied to it as held
// (appliedTranscript), and read whole again only when one can't be. last:
// only its latest entries are held.
export function useTranscript(slug: string, id: string, last?: number): { t?: Transcript; error?: string; reload: () => void } {
  const [t, setT] = useState<Transcript>();
  // held is what's held, as of now: a change is applied to it as it's told,
  // not when the page next draws.
  const held = useRef<Transcript>(undefined);
  const put = useCallback((x: Transcript | undefined) => {
    held.current = x;
    setT(x);
  }, []);
  const [error, setError] = useState<string>();
  const visible = useVisible();
  const begin = useLatest();
  const reload = useCallback(() => {
    const fresh = begin();
    getTranscript(slug, id, last).then(
      (x) => {
        if (!fresh()) return;
        // A read that crossed a change told after it is older than what's held.
        if (held.current?.available && x.version < (held.current.version ?? 0)) return;
        put(x);
        setError(undefined);
      },
      (e: Error) => fresh() && setError(e.message),
    );
  }, [slug, id, last, begin, put]);
  // Another crew's: a read of the last one's, under way, is stale, and
  // what's held is the last one's.
  useEffect(() => {
    begin();
    put(undefined);
  }, [slug, id, begin, put]);
  useEffect(() => {
    if (!visible) return;
    return pageSocket.follow(`transcript:${slug}/${id}`, {
      following: reload,
      changed: (said) => {
        const next = said.transcript ? appliedTranscript(held.current, said.transcript, last) : "read";
        if (next === "read") reload();
        else if (next) put(next);
      },
      lost: (closed) => closed && reload(),
    });
  }, [slug, id, last, visible, reload, put]);
  return { t, error, reload };
}
