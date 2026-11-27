// Your automations' actions, where you act on things: a seeded try's
// (scripts/seeds/actions.sh, on plugin-refs). Find under Do, Find's Here on
// a mission, a mission's row's menu, and what an action asks before it
// runs.
import { defineFlow } from "shotta";

export default defineFlow({
  shows: "Running your automations' actions from Find and a row's menu",
  video: false,
  steps: [
    { goto: "/" },
    { press: "ControlOrMeta+k" },
    { type: "digest" },
    { shot: "find-do", viewport: true, shows: "Find, asked for digest: Write today's digest, an automation's action on nothing in particular" },
    { press: "Escape" },
    { goto: "/" },
    {
      label: "the mission's row, right clicked",
      run: async (page) => {
        await page.locator("text=Shelve the lantern").first().click({ button: "right" });
        await page.waitForTimeout(500);
      },
    },
    { shot: "row-menu", viewport: true, shows: "A mission's row's menu: what ws does to it, then your automation's Note where it's at" },
    { press: "Escape" },
    { goto: "/" },
    { press: "ControlOrMeta+k" },
    { type: "THING-2" },
    { press: "ControlOrMeta+k" },
    { shot: "acting", viewport: true, shows: "Acting on THING-2 from Find: the plugin's actions, ws's, then your automations' Ask after it and Dust it" },
    { type: "ask after" },
    { press: "Enter" },
    {
      label: "its dialog",
      run: async (page) => {
        await page.waitForTimeout(500);
      },
    },
    { shot: "asks", viewport: true, shows: "What Ask after it asks before it runs: a question, and how" },
    { type: "Is it lit?" },
    { click: 'role=button[name="Run"]' },
    {
      label: "its toast",
      run: async (page) => {
        await page.waitForTimeout(1500);
      },
    },
    { shot: "ran", viewport: true, shows: "What it did, said once it ran: its notification's words" },
  ],
});
