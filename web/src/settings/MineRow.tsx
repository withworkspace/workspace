import { Link } from "react-router";
import { Button } from "../ui/Button";
import { type Mine, name, valueSaid } from "./sections";
import { SettingsKey } from "./SettingsKey";
import { SettingsMineRow, SettingsMineValue, SettingsMineWhat } from "./SettingsMine";
import { useSetting, without } from "./useSetting";
import { Text, textClass } from "../ui/Text";

// MineRow is a setting your file sets: which it is, what it's set to and
// what it would be otherwise, and Reset, which takes yours away.
export function MineRow({ m }: { m: Mine }) {
  const { reset } = useSetting(m.s);
  const plain = m.s.kind === "bool" || m.s.kind === "choice";
  return (
    <SettingsMineRow>
      <SettingsMineWhat>
        <span>
          <Link to={`/settings/${m.section.id}`} className={textClass("link")}>
            {m.section.title}
          </Link>{" "}
          · {m.what}
        </span>
        <SettingsKey>{name(m.s)}</SettingsKey>
      </SettingsMineWhat>
      <SettingsMineValue>
        <span className={plain ? undefined : textClass("mono")}>{valueSaid(m.s)}</span>
        <Text dim>otherwise {without(m.s)}</Text>
      </SettingsMineValue>
      <Button variant="ghost" size="small" onClick={reset}>
        Reset
      </Button>
    </SettingsMineRow>
  );
}
