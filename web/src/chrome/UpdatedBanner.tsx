import { useRef } from "react";
import { shortVersion } from "../shell/version";
import { WindowUpdated } from "./WindowUpdated";

// UpdatedBanner says when the daemon serving the page is another ws than
// the one the page was loaded from: it was upgraded, and restarted onto the
// new one, while the page stayed open. The page is the old ws's web UI,
// which the new one may not answer as it expects: reloading it loads the
// new one's.
export function UpdatedBanner({ version }: { version?: string }) {
  const loaded = useRef<string>(undefined);
  if (version && loaded.current === undefined) loaded.current = version;
  if (!version || !loaded.current || version === loaded.current) return null;
  return <WindowUpdated version={shortVersion(version)} onReload={() => location.reload()} />;
}
