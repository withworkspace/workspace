import { SettingsSource } from "./SettingsSource";

// BrowserSource says a row is kept in this browser, where a file's
// setting says whose it is.
export function BrowserSource() {
  return <SettingsSource title="Kept in this browser, not in a file">this browser</SettingsSource>;
}
