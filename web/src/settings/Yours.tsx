import { Link } from "react-router";
import type { SettingsInfo } from "../api";
import { Keys } from "../ui/Keys";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { usePref } from "../prefs";
import { useTheme } from "../ui/theme";
import { fileName } from "./file";
import { MineRow } from "./MineRow";
import { usePage } from "./page";
import { minesOf, name, type Section } from "./sections";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsMine, SettingsMineRow } from "./SettingsMine";
import { SettingsUnknown } from "./SettingsUnknown";
import { textClass } from "../ui/Text";

// Yours is what's yours alone: what your file sets, each with what it
// would be without yours and the way back; the keys ws doesn't read; and
// what this browser keeps.
export function Yours({ info, sections }: { info: SettingsInfo; sections: Section[] }) {
  const { notify } = usePage();
  const { choice, depth } = useTheme();
  const sendKey = useSendKey();
  const [keyHints] = usePref("keyHints");
  const [keymap] = usePref("keymap");
  const shortcuts = Object.keys(keymap).length;
  const mines = minesOf(sections);
  return (
    <>
      {info.unknown && info.unknown.length > 0 && <SettingsUnknown keys={info.unknown} />}
      <SettingsBlock title={<>Set in {fileName(info.file)}</>} count={mines.length}>
        {mines.length === 0 && <div className="quiet-line">Nothing: all settings are the team's or defaults.</div>}
        <SettingsMine>
          {mines.map((m) => (
            <MineRow key={name(m.s)} m={m} />
          ))}
        </SettingsMine>
      </SettingsBlock>
      <SettingsBlock title="In this browser">
        <SettingsMine>
          <SettingsMineRow>
            <span>
              <Link to="/settings/appearance" className={textClass("link")}>
                Appearance
              </Link>{" "}
              · Theme
            </span>
            <span>{{ system: "System", light: "Light", dark: "Dark" }[choice]}</span>
          </SettingsMineRow>
          <SettingsMineRow>
            <span>
              <Link to="/settings/appearance" className={textClass("link")}>
                Appearance
              </Link>{" "}
              · Depth
            </span>
            <span>{depth === 0 ? "Default" : depth === 100 ? "Black" : `${depth}%`}</span>
          </SettingsMineRow>
          <SettingsMineRow>
            <span>
              <Link to="/settings/keyboard" className={textClass("link")}>
                Keyboard
              </Link>{" "}
              · Send with
            </span>
            <span>
              <Keys k={sendKeys(sendKey)} /> sends
            </span>
          </SettingsMineRow>
          <SettingsMineRow>
            <span>
              <Link to="/settings/keyboard" className={textClass("link")}>
                Keyboard
              </Link>{" "}
              · Key hints
            </span>
            <span>{keyHints ? "On" : "Off"}</span>
          </SettingsMineRow>
          <SettingsMineRow>
            <span>
              <Link to="/settings/keyboard" className={textClass("link")}>
                Keyboard
              </Link>{" "}
              · Shortcuts
            </span>
            <span>{shortcuts === 0 ? "Their own keys" : shortcuts === 1 ? "1 set by you" : `${shortcuts} set by you`}</span>
          </SettingsMineRow>
          <SettingsMineRow>
            <span>
              <Link to="/settings/notifications" className={textClass("link")}>
                Notifications
              </Link>{" "}
              · Desktop notifications
            </span>
            <span>{notify.on ? "On" : "Off"}</span>
          </SettingsMineRow>
        </SettingsMine>
      </SettingsBlock>
    </>
  );
}
