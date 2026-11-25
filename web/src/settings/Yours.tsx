import { Link } from "react-router";
import type { SettingsInfo } from "../api";
import { useTheme } from "../ui/theme";
import { fileName } from "./file";
import { MineRow } from "./MineRow";
import { usePage } from "./page";
import { minesOf, name, type Section } from "./sections";
import { SettingsBlock } from "./SettingsBlock";
import { SettingsMine, SettingsMineRow } from "./SettingsMine";
import { SettingsUnknown } from "./SettingsUnknown";

// Yours is what's yours alone: what your file sets, each with what it
// would be without yours and the way back; the keys ws doesn't read; and
// what this browser keeps.
export function Yours({ info, sections }: { info: SettingsInfo; sections: Section[] }) {
  const { notify } = usePage();
  const { choice } = useTheme();
  const mines = minesOf(sections);
  return (
    <>
      {info.unknown && info.unknown.length > 0 && <SettingsUnknown keys={info.unknown} />}
      <SettingsBlock title={<>What {fileName(info.file)} sets</>} count={mines.length}>
        {mines.length === 0 && <div className="quiet-line">Nothing: every setting is the team's, or ws's default.</div>}
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
              <Link to="/settings/appearance" className="link">
                Appearance
              </Link>{" "}
              · Theme
            </span>
            <span>{{ system: "As the system has it", light: "Light", dark: "Dark" }[choice]}</span>
          </SettingsMineRow>
          <SettingsMineRow>
            <span>
              <Link to="/settings/notifications" className="link">
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
