import { Switch } from "../ui/Switch";
import { BrowserSource } from "./BrowserSource";
import { Marked } from "./Marked";
import { usePage } from "./page";
import { words } from "./sections";
import { SettingsRow } from "./SettingsRow";

// NotifyRow is desktop notifications, on or off: the bell beside Needs
// you in the sidebar is the same switch.
export function NotifyRow() {
  const { notify } = usePage();
  const blocked = notify.supported && !notify.on && Notification.permission === "denied";
  return (
    <SettingsRow
      label={<Marked text={words.notify.label} />}
      about={
        <>
          <Marked text={words.notify.about} />
          {!notify.supported && " Not supported in this browser."}
          {blocked && " Blocked by the browser: allow notifications in browser settings."}
        </>
      }
      source={<BrowserSource />}
    >
      <Switch on={notify.on} disabled={!notify.supported} onChange={notify.toggle} aria-label={words.notify.label} />
    </SettingsRow>
  );
}
