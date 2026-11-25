import { setPref } from "../prefs";
import { Keys } from "../ui/Keys";
import { Segment, Segmented } from "../ui/Segmented";
import { type SendKey, sendKeys, useSendKey } from "../ui/sendKey";
import { BrowserSource } from "./BrowserSource";
import { Marked } from "./Marked";
import { words } from "./sections";
import { SettingsRow } from "./SettingsRow";

const choices: SendKey[] = ["mod+enter", "enter"];

// SendKeyRow is the key that sends what you write in a box (ui/sendKey.ts):
// this is where it's chosen. It says what a new line is with the one
// you have.
export function SendKeyRow() {
  const key = useSendKey();
  return (
    <SettingsRow
      label={<Marked text={words.sendKey.label} />}
      about={
        <>
          <Marked text={words.sendKey.about} /> <Keys k={key === "enter" ? "shift+⏎" : "⏎"} /> is a new line.
        </>
      }
      source={<BrowserSource />}
    >
      <Segmented value={key} onChange={(to) => setPref("sendKey", to)} aria-label={words.sendKey.label}>
        {choices.map((c) => (
          <Segment key={c} value={c}>
            <Keys k={sendKeys(c)} /> sends
          </Segment>
        ))}
      </Segmented>
    </SettingsRow>
  );
}
