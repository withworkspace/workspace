import type { StorybookConfig } from "@storybook/react-vite";

// Storybook draws parts of the web UI alone, from missions ws gave
// (src/mission/fixtures, which scripts/story-fixtures.mjs takes again):
// for looking at every state a part has, side by side. `just stories`.
const config: StorybookConfig = {
  framework: "@storybook/react-vite",
  stories: ["../src/**/*.stories.tsx"],
  // Its docs read props with the TypeScript compiler's API, which isn't
  // TypeScript 7's; nothing here is documented that way.
  typescript: { reactDocgen: false },
  // It says nothing of this repo to anyone.
  core: { disableTelemetry: true, disableWhatsNewNotifications: true },
};

export default config;
