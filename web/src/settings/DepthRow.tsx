import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { setDepth, showDepth, useTheme } from "../ui/theme";
import { BrowserSource } from "./BrowserSource";
import { Marked } from "./Marked";
import { words } from "./sections";
import { SettingsLevel } from "./SettingsLevel";
import { SettingsRow } from "./SettingsRow";

// DepthRow is how deep the dark theme goes, towards black for an OLED
// screen: kept in this browser, as the theme is. The page follows as it's
// dragged, and it's kept once let go; left mid-drag, it goes back.
export function DepthRow() {
  const { depth } = useTheme();
  const [preview, setPreview] = useState<number | null>(null);
  const value = preview ?? depth;
  const slider = useRef<HTMLInputElement>(null);

  useLayoutEffect(() => showDepth(value), [value]);

  // The slider's own change comes when it's let go (React's onChange comes
  // with every move).
  useEffect(() => {
    const el = slider.current;
    if (!el) return;
    const changed = () => {
      setDepth(Number(el.value));
      setPreview(null);
    };
    el.addEventListener("change", changed);
    return () => el.removeEventListener("change", changed);
  }, []);

  const kept = useRef(depth);
  kept.current = depth;
  useEffect(() => () => showDepth(kept.current), []);

  return (
    <SettingsRow label={<Marked text={words.depth.label} />} about={<Marked text={words.depth.about} />} source={<BrowserSource />}>
      <SettingsLevel label={words.depth.label} ends={["Default", "Black"]} value={value} slider={slider} onPreview={setPreview} />
    </SettingsRow>
  );
}
