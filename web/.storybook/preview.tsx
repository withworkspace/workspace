import type { Preview } from "@storybook/react-vite";
import { MemoryRouter } from "react-router";
import "../src/styles.css";
import { stubApi } from "../src/stories/api";
import { setClock } from "../src/stories/clock";
import taken from "../src/mission/fixtures/taken.json";

// The stories' clock is when their missions were taken, and their API
// answers from what was taken then.
setClock(taken.at);
stubApi();

const preview: Preview = {
  globalTypes: {
    theme: {
      description: "ws's theme",
      toolbar: {
        title: "Theme",
        icon: "mirror",
        items: [
          { value: "light", title: "Light" },
          { value: "dark", title: "Dark" },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { theme: "light" },
  parameters: { layout: "fullscreen" },
  decorators: [
    (Story, context) => {
      // As public/theme.js and src/ui/theme.ts set it, on <html>.
      document.documentElement.setAttribute("data-theme", context.globals.theme === "dark" ? "dark" : "light");
      return (
        <MemoryRouter>
          <Story />
        </MemoryRouter>
      );
    },
  ],
};

export default preview;
