import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { WindowError } from "./WindowError";
import { WindowKeyLabel } from "./WindowKeyLabel";
import { WindowKeys } from "./WindowKeys";
import { WindowLost, windowStaleClass } from "./WindowLost";
import { WindowMenu } from "./WindowMenu";
import { WindowNotFound } from "./WindowNotFound";
import { WindowNotice } from "./WindowNotice";
import { WindowUpdated } from "./WindowUpdated";
import { WorkspaceName } from "./WorkspaceName";
import { Text } from "../ui/Text";

// What the window draws of its own, round the page: the workspace's menu
// (press its name), the banners across the top (the daemon lost, with the
// page under it dimmed; ws updated; the workspace not loaded), the keys,
// a page that isn't there, and the page in place of the window when the
// daemon's closed or this isn't signed in.

const meta: Meta = { title: "Chrome/Window" };
export default meta;

function Said({ children }: { children: (say: (what: string) => void) => React.ReactNode }) {
  const [said, say] = useState("Nothing chosen yet.");
  return (
    <div style={{ display: "flex", gap: 16, alignItems: "center", padding: 24 }}>
      {children(say)}
      <Text dim>{said}</Text>
    </div>
  );
}

const places = [
  { id: "runs", label: "Runs", icon: "play" as const, current: true },
  { id: "reviews", label: "Reviews", icon: "pr" as const, count: 2, countTitle: "to review" },
  { id: "team", label: "Team", icon: "user" as const },
];

// The menu with places moved into it (the one you're on, one with a
// count), a later release out, and the version at its foot; and the menu
// with nothing moved into it and nothing new.
export const Menu: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 8 }}>
      {[true, false].map((full) => (
        <Said key={String(full)}>
          {(say) => (
            <div style={{ width: 220 }}>
              <WindowMenu
                trigger={<WorkspaceName name="sorrel" root="~/sorrel" wants={full ? "Reviews: 2 to review" : undefined} />}
                places={full ? places : []}
                version={{ full: "v0.16.0-12-g3f2a91c", short: "0.16.0+12" }}
                later={full ? "0.17.0" : undefined}
                onPlace={(id) => say(`Went to ${id}.`)}
                onSettings={() => say("Settings.")}
                onKeys={() => say("Keys.")}
                onLater={() => say("To Settings › About.")}
                onShutDown={() => say("Asked to shut down.")}
              />
            </div>
          )}
        </Said>
      ))}
    </div>
  ),
};

// Across the top of the page: the daemon lost (what's under it dimmed, as
// the page is while it's lost), ws updated under it, and the workspace
// that didn't load.
export const Banners: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 24 }}>
      <main className={windowStaleClass} style={{ height: 160, border: "1px dashed var(--line)" }}>
        <WindowLost for="2m 10s" />
        <div style={{ padding: 24 }}>What's on the page, dimmed: it may be out of date.</div>
      </main>
      <WindowUpdated version="0.17.0" onReload={() => {}} />
      <WindowError>Couldn't read the workspace: the daemon answered 500.</WindowError>
    </div>
  ),
};

// The keys: the page's and those everywhere, each label in the forms the
// lists write: one key, alternatives, a sequence, a chord.
export const Keys: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gap: 24, padding: 24, maxWidth: 560 }}>
      <WindowKeys
        groups={[
          {
            title: "On this page",
            keys: [
              ["j / k", "Move down and up the runs"],
              ["⏎ or o", "Open the one the cursor's on"],
            ],
          },
          {
            title: "Everywhere",
            keys: [
              ["mod+K", "Find anything: a mission, a file, a pull request, an address"],
              ["g m", "Go to Missions"],
              ["?", "These keys"],
            ],
          },
        ]}
      />
      <div style={{ display: "flex", gap: 16 }}>
        <WindowKeyLabel k="?" />
        <WindowKeyLabel k="j / k" />
        <WindowKeyLabel k="g d / g p" />
        <WindowKeyLabel k="mod+shift+F" />
      </div>
    </div>
  ),
};

export const NotFound: StoryObj = { name: "Not found", render: () => <WindowNotFound /> };

// In place of the window: the daemon closed from here, and a page that
// isn't signed in.
export const Notices: StoryObj = {
  render: () => (
    <div style={{ display: "grid" }}>
      <WindowNotice title="The daemon is closed">
        Its crew were hung up and its web UI went with it. Run <code>ws web</code> in the workspace to start it again.
      </WindowNotice>
      <WindowNotice title="Not signed in">
        This page signs in through a link from your terminal. Run <code>ws web</code> in the workspace.
      </WindowNotice>
    </div>
  ),
};
