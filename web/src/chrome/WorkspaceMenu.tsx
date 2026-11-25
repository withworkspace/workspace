import { useLocation, useNavigate } from "react-router";
import { askShutDown } from "../shell/daemon";
import type { Place } from "../shell/places";
import { shortVersion } from "../shell/version";
import { useWorkspaceIfRead } from "../workspace/store";
import { useChromeStore } from "./store";
import { WindowMenu } from "./WindowMenu";
import { WorkspaceName } from "./WorkspaceName";

// WorkspaceMenu is the workspace's name, opening what's of the workspace
// rather than a place in it: Settings (where the theme is, and the way to
// arrange the sidebar), Keys, and shutting its daemon down, which
// otherwise stays open until it's closed. Places moved out of the sidebar
// are at its top. At its foot is what this is and which: the logotype, and
// the version of ws that serves the page. It's drawn as a WindowMenu.
export function WorkspaceMenu({ places }: { places: Place[] }) {
  // What the menu says of the workspace, each read by itself: it isn't
  // drawn again for anything else that changes.
  const name = useWorkspaceIfRead((w) => w.name) ?? "ws";
  const root = useWorkspaceIfRead((w) => w.root);
  const version = useWorkspaceIfRead((w) => w.version);
  const later = useWorkspaceIfRead((w) => w.later);
  const navigate = useNavigate();
  const { pathname } = useLocation();
  // A place in the menu that wants you says so on the name, as the
  // sidebar's would.
  const counted = places.filter((p) => p.count);
  return (
    <WindowMenu
      trigger={<WorkspaceName name={name} root={root} wants={counted.length > 0 ? counted.map((p) => `${p.label}: ${p.count} ${p.countTitle ?? "need you"}`).join("\n") : undefined} />}
      places={places.map((p) => ({ id: p.id, label: p.label, icon: p.icon, count: p.count, countTitle: p.countTitle, keys: p.keys, current: p.match(pathname) }))}
      version={version ? { full: version, short: shortVersion(version) } : undefined}
      later={later}
      onPlace={(id) => {
        const p = places.find((x) => x.id === id);
        if (p) navigate(p.to);
      }}
      onSettings={() => navigate("/settings")}
      onKeys={() => useChromeStore.getState().setHelp(true)}
      onLater={() => navigate("/settings/about#updates")}
      onShutDown={() => void askShutDown()}
    />
  );
}
