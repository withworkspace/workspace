import { DaemonRow } from "./DaemonRow";
import { KeyRow } from "./KeyRow";
import { NotifyRow } from "./NotifyRow";
import { PlacesRow } from "./PlacesRow";
import type { Row } from "./sections";
import { SendKeyRow } from "./SendKeyRow";
import { ThemeRow } from "./ThemeRow";

// RowOf draws a row of the Settings page, whichever it is: a setting a
// file has a key for, or one of the page's own (the theme, the key that
// sends, notifications, the sidebar, the daemon), all drawn as a SettingsRow. found says it's
// out of its block (Find's results), where a setting says whose it is.
export function RowOf({ row, found }: { row: Row; found?: boolean }) {
  switch (row.kind) {
    case "setting":
      return <KeyRow s={row.s} crumb={found ? row.crumb : undefined} />;
    case "places":
      return <PlacesRow settings={row.settings} />;
    case "theme":
      return <ThemeRow />;
    case "sendKey":
      return <SendKeyRow />;
    case "notify":
      return <NotifyRow />;
    case "daemon":
      return <DaemonRow />;
  }
}
