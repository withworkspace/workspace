import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { showTint } from "../ui/accent";
import type { ControlProps } from "./controlProps";
import { SettingsLevel } from "./SettingsLevel";

// TintInput is the tint setting: how far the accent tints the greys. The
// page follows as you drag (the tint on <html>), and it's written once you
// let go; one that isn't kept (the page left first) goes back to what's
// set. Back at 0, yours is taken away, unless the team's is set, which
// only yours at 0 is over.
export function TintInput({ s, set, reset }: ControlProps) {
  const saved = typeof s.value === "number" ? s.value : 0;
  const [preview, setPreview] = useState<number | null>(null);
  const value = preview ?? saved;
  const slider = useRef<HTMLInputElement>(null);
  const writing = useRef(false);

  useLayoutEffect(() => showTint(value), [value]);

  const keep = (n: number) => {
    writing.current = true;
    const done = () => {
      writing.current = false;
      setPreview(null);
    };
    if (n === 0 && (s.team === undefined || s.team === null)) {
      if (s.source === "yours") reset();
      done();
      return;
    }
    void set(n).then(done);
  };

  // The slider's own change comes when it's let go (React's onChange comes
  // with every move).
  const keepRef = useRef(keep);
  keepRef.current = keep;
  useEffect(() => {
    const el = slider.current;
    if (!el) return;
    const changed = () => keepRef.current(Number(el.value));
    el.addEventListener("change", changed);
    return () => el.removeEventListener("change", changed);
  }, []);

  // Left mid-drag, the page goes back to what's set.
  const kept = useRef(saved);
  kept.current = saved;
  useEffect(
    () => () => {
      if (!writing.current) showTint(kept.current);
    },
    [],
  );

  return <SettingsLevel label={s.label} ends={["Grey", "Tinted"]} value={value} slider={slider} onPreview={setPreview} />;
}
