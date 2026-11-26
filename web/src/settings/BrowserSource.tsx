import { SettingsSource } from "./SettingsSource";

// BrowserSource says a row is kept in this browser, where a file's
// setting says whose it is.
export function BrowserSource() {
  return <SettingsSource title="Saved in this browser only">this browser</SettingsSource>;
}
