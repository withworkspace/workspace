import { DaemonRow } from "./DaemonRow";
import { DepthRow } from "./DepthRow";
import { KeyHintsRow } from "./KeyHintsRow";
import { KeyRow } from "./KeyRow";
import { NotifyRow } from "./NotifyRow";
import { PlacesRow } from "./PlacesRow";
import type { Row } from "./sections";
import { SendKeyRow } from "./SendKeyRow";
import { ThemeRow } from "./ThemeRow";

// RowOf draws a row of the Settings page, whichever it is: a setting a
// file has a key for, or one of the page's own (the theme, its depth, the key that
// sends, key hints, notifications, the sidebar, the daemon), all drawn as a SettingsRow. found says it's
// out of its block (Find's results), where a setting says whose it is.
export function RowOf({ row, found }: { row: Row; found?: boolean }) {
  switch (row.kind) {
    case "setting":
      return <KeyRow s={row.s} crumb={found ? row.crumb : undefined} />;
    case "places":
      return <PlacesRow settings={row.settings} />;
    case "theme":
      return <ThemeRow />;
    case "depth":
      return <DepthRow />;
    case "sendKey":
      return <SendKeyRow />;
    case "keyHints":
      return <KeyHintsRow />;
    case "notify":
      return <NotifyRow />;
    case "daemon":
      return <DaemonRow />;
  }
}
