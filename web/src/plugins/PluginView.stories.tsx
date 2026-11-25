import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";
import { PluginView } from "./PluginView";

// A plugin's view, from its part: the app's frame filling the page, and
// ws's word above it where it couldn't meet the app.

const meta: Meta = { title: "Plugins/PluginView" };
export default meta;

const app = `<body style="margin:0;font:14px system-ui;padding:16px;background:#8881">The plugin's app, in its frame.</body>`;

function Page({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div style={{ display: "grid", gap: 6 }}>
      <strong>{title}</strong>
      <div style={{ position: "relative", height: 220, border: "1px dashed var(--line)" }}>{children}</div>
    </div>
  );
}

export const States: StoryObj = {
  render: () => (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, padding: 24 }}>
      <Page title="The app">
        <PluginView>
          <iframe title="Frame fixture" srcDoc={app} />
        </PluginView>
      </Page>
      <Page title="Refused">
        <PluginView refused="frame-fixture's page speaks frame protocol 2, and this ws only 1: update ws, or the plugin.">
          <iframe title="Frame fixture" srcDoc={app} />
        </PluginView>
      </Page>
    </div>
  ),
};
