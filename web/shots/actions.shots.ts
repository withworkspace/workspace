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
    { goto: "/" },
    { press: "ControlOrMeta+k" },
    { type: "THING-2" },
    { press: "ControlOrMeta+k" },
    { type: "dust" },
    { press: "Enter" },
    {
      label: "its card",
      run: async (page) => {
        await page.waitForTimeout(2500);
      },
    },
    { shot: "act-confirm", viewport: true, shows: "Dust it, run by you: what it'd do through the plugin, its card, in front of you to accept or decline" },
    { click: 'role=dialog >> role=button[name="Accept"]' },
    {
      label: "done",
      run: async (page) => {
        await page.getByText("Dust it: Done.").waitFor();
      },
    },
    { shot: "act-done", viewport: true, shows: "Accepted: done, and said so" },
    { goto: "/automations/moved-notes" },
    { shot: "grants", viewport: true, shows: "moved-notes's page: Through plugins, let do Fixture things without asking, and its run that did" },
    { goto: "/x/frame-fixture/" },
    {
      label: "THING-2, right clicked in the plugin's app",
      run: async (page) => {
        const app = page.frameLocator("iframe");
        await app.locator(".ws-ref", { hasText: "THING-2" }).first().click({ button: "right" });
        await page.waitForTimeout(600);
      },
    },
    { shot: "frame-menu", viewport: true, shows: "A right click on THING-2 in a plugin's app: the app's own Polish THING-2, then your automations' actions on it, in ws's menu over the frame" },
    { click: 'role=menuitem[name="Polish THING-2"]' },
    {
      label: "the app's count",
      run: async (page) => {
        await page.waitForTimeout(500);
      },
    },
    { shot: "polished", viewport: true, shows: "The app's own entry, chosen in ws's menu, run in the app: Polished, from ws's menu, goes up" },
  ],
});
