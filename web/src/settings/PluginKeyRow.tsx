import { useEffect, useRef, useState } from "react";
import { keyName } from "../keys";
import { holdKeys } from "../keymap/dispatch";
import { usePref } from "../prefs";
import { BrowserSource } from "./BrowserSource";
import { Marked } from "./Marked";
import { words } from "./sections";
import { SettingsKeyPick } from "./SettingsKeyPick";
import { SettingsRow } from "./SettingsRow";

// PluginKeyRow is the key plugins' suggested keys go after (prefs.ts,
// pluginKey): pressed to set it, as a shortcut's key is (keymap/dispatch.ts,
// holdKeys). esc gives up.
export function PluginKeyRow() {
  const [key, setKey] = usePref("pluginKey");
  const [picking, setPicking] = useState(false);
  const release = useRef<() => void>(undefined);
  const stop = () => {
    release.current?.();
    release.current = undefined;
    setPicking(false);
  };
  useEffect(() => stop, []);
  const pick = () => {
    stop();
    setPicking(true);
    release.current = holdKeys((e) => {
      const k = keyName(e);
      if (k !== "Escape") setKey(k);
      stop();
    });
  };
  return (
    <SettingsRow label={<Marked text={words.pluginKey.label} />} about={<Marked text={words.pluginKey.about} />} source={<BrowserSource />}>
      <SettingsKeyPick value={key} picking={picking} onPick={pick} onClear={() => setKey("")} onCancel={stop} />
    </SettingsRow>
  );
}
