import { useCallback, useEffect, useRef, useState } from "react";
import { getRunLog, useVisible } from "../api";
import { useLatest } from "../load";
import { pageSocket } from "../socket";
import type { RunLog, RunLogOp } from "../wire.gen";

// logLines is how many of a log's last lines a page holds, as the server
// sends them (web.runLogLines).
export const logLines = 400;

// appliedRunLog is a run's log as held with what's been written told over
// the socket: undefined when it's in it already, and "read" when it can't
// be followed on (it's held as of more or less than the change is from).
export function appliedRunLog(held: RunLog | undefined, op: RunLogOp): RunLog | undefined | "read" {
  if (!held) return "read";
  if (op.size <= held.size) return undefined;
  if (op.from !== held.size) return "read";
  const lines = (held.log + op.text).split("\n");
  // The last is what follows the last newline: a line not yet ended, or "".
  const keep = lines.length - 1 > logLines ? lines.slice(lines.length - 1 - logLines) : lines;
  return { ref: held.ref, log: keep.join("\n"), size: op.size };
}

// useRunLog is a run's output: read once, then, while it runs and the page
// is showing, followed over the tab's socket (runlog:<ref>), told what's
// written to it. It's read again as it ends, for its last words.
export function useRunLog(ref: string, live: boolean): { value?: RunLog; error?: string } {
  const [value, setValue] = useState<RunLog>();
  const [error, setError] = useState<string>();
  const held = useRef<RunLog>(undefined);
  const visible = useVisible();
  const begin = useLatest();
  const put = useCallback((x: RunLog | undefined) => {
    held.current = x;
    setValue(x);
  }, []);
  const read = useCallback(() => {
    const fresh = begin();
    getRunLog(ref).then(
      (x) => {
        if (!fresh() || (held.current && x.size < held.current.size && held.current.ref === x.ref)) return;
        put(x);
        setError(undefined);
      },
      (e: Error) => fresh() && setError(e.message),
    );
  }, [ref, begin, put]);
  // Read now, unless it's followed: following it reads it.
  const followed = live && visible;
  useEffect(() => {
    put(undefined);
    if (!followed) read();
  }, [ref, followed, read, put]);
  useEffect(() => {
    if (!live || !visible) return;
    return pageSocket.follow(`runlog:${ref}`, {
      following: read,
      changed: (said) => {
        const next = said.runLog ? appliedRunLog(held.current, said.runLog) : "read";
        if (next === "read") read();
        else if (next) put(next);
      },
      lost: (closed) => closed && read(),
    });
  }, [ref, live, visible, read, put]);
  return { value, error };
}
