import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { defaultAccent, parseHex, pulled, showAccent, shownAs } from "../ui/accent";
import { useTheme } from "../ui/theme";
import type { ControlProps } from "./controlProps";
import { SettingsAccent } from "./SettingsAccent";

// Accents to pick from, beside ws's own and any other: each reads in
// either theme much as it's picked.
const accents: [hex: string, name: string][] = [
  ["#4a5bd0", "Indigo"],
  ["#2b6cb8", "Blue"],
  ["#0e7f86", "Teal"],
  ["#2f7d4f", "Green"],
  ["#a8650f", "Amber"],
  ["#b8456f", "Rose"],
  ["#59626f", "Slate"],
];

// AccentInput is the accent setting: ws's own and a few others to pick
// from, a colour picker for any other, and the hex it's at, any of which
// changes it. The page follows as you pick (the accent on <html>), and the
// pick's written once you let it go; one that isn't kept (refused, or the
// page left first) goes back to what's set. A pick the theme lightens or
// darkens so it reads on the page shows as it's drawn, and says so.
export function AccentInput({ s, set, reset }: ControlProps) {
  const saved = typeof s.value === "string" ? s.value : null;
  const [ours] = useState(defaultAccent);
  const [preview, setPreview] = useState<string | null>(null);
  const picked = preview ?? saved ?? ours;
  const [editing, setEditing] = useState<string | null>(null);
  const { dark } = useTheme();
  const [themed, setThemed] = useState("");
  const picker = useRef<HTMLInputElement>(null);
  const waiting = useRef<number>(undefined);
  const writing = useRef(false);
  const cancelled = useRef(false);

  // The page wears what's being picked, else what's set, before it paints.
  useLayoutEffect(() => {
    showAccent(preview ?? saved);
    setThemed(shownAs("--accent"));
  }, [preview, saved, dark]);

  // keep writes a pick; the page then wears what's set.
  const keep = (v: string, wait: number) => {
    window.clearTimeout(waiting.current);
    waiting.current = window.setTimeout(() => {
      writing.current = true;
      void set(v).then(() => {
        writing.current = false;
        setPreview(null);
      });
    }, wait);
  };

  // The picker's own change comes when it's let go (React's onChange comes
  // with every move). Safari's can come as it moves, so a pick is written
  // once it's been still a moment.
  const keepRef = useRef(keep);
  keepRef.current = keep;
  useEffect(() => {
    const el = picker.current;
    if (!el) return;
    const changed = () => keepRef.current(el.value, 400);
    el.addEventListener("change", changed);
    return () => el.removeEventListener("change", changed);
  }, []);

  // Left mid-pick, the page goes back to what's set; left while a pick's
  // being written, it keeps the pick.
  const kept = useRef(saved);
  kept.current = saved;
  useEffect(
    () => () => {
      window.clearTimeout(waiting.current);
      if (!writing.current) showAccent(kept.current);
    },
    [],
  );

  // typed writes the hex typed, once it's left: what isn't a hex colour
  // goes to be refused, saying why.
  const typed = (text: string) => {
    setEditing(null);
    const t = text.trim();
    const hex = parseHex(t);
    if (cancelled.current || t === "" || hex === (saved ?? ours)) {
      cancelled.current = false;
      setPreview(null);
      return;
    }
    keep(hex ?? t, 0);
  };

  // ws's own is no accent set: yours taken away, unless the team has one,
  // which only ws's own, set as yours, is over.
  const pickOurs = () => {
    if (s.team !== undefined && s.team !== null) keep(ours, 0);
    else if (s.source === "yours") reset();
  };
  const way = pulled(picked, themed);

  return (
    <SettingsAccent
      label={s.label}
      ours={ours}
      accents={accents}
      picked={picked}
      hex={editing ?? picked}
      dark={dark}
      pulled={way ? { way, as: themed } : undefined}
      picker={picker}
      onOurs={pickOurs}
      onPick={(hex) => keep(hex, 0)}
      onPreview={setPreview}
      onHexFocus={() => setEditing(picked)}
      onHexChange={(text) => {
        setEditing(text);
        const hex = parseHex(text);
        if (hex) setPreview(hex);
      }}
      onHexBlur={typed}
      onHexCancel={() => {
        cancelled.current = true;
      }}
    />
  );
}
