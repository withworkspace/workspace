// How the web UI is looked at, and its pictures taken: shotta drives a
// headless browser through a try's web UI (https://github.com/brudil/shotta).
//
//   pnpm shotta snap /team --agent     a page, light and dark, to look at
//   pnpm shotta export                 the README's and FEATURES.md's pictures
//
// (just shots makes the try the pictures are of, and tears it down; AGENTS.md
// says how to look at a change.)
import { execFileSync } from "node:child_process";
import { join } from "node:path";
import { defineConfig } from "shotta";

// The try's sign-in link: where its web UI is, and what signs a browser in.
// WS_LINK says which; otherwise it's this checkout's try, as scripts/try.sh
// knows it. With no try up there's nothing to open, which shotta says when
// it's asked to (the address is one nothing answers at); checking the flows
// needs none.
function tryLink(): URL {
  if (process.env.WS_LINK) return new URL(process.env.WS_LINK);
  try {
    const link = execFileSync(join(import.meta.dirname, "../scripts/try.sh"), ["info", "link"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] });
    return new URL(link.trim());
  } catch {
    return new URL("http://127.0.0.1:9/auth");
  }
}
const link = tryLink();

// The page's size, and how many pixels a point of it is drawn with: a
// README's column is about 1000 points wide, so this is sharp there.
const page = { viewport: { width: 1280, height: 800 }, deviceScaleFactor: 2 };

// A picture's two files, light and dark, as an export names them.
const pictures = (shots: Record<string, string>) => Object.fromEntries(Object.entries(shots).flatMap(([name, shot]) => ["light", "dark"].map((theme) => [`${name}-${theme}.webp`, `${shot}@${theme}`])));

export default defineConfig({
  baseUrl: link.origin,
  // Light and dark are what a look takes; the phone is there to ask for (@phone).
  profiles: {
    light: { ...page, colorScheme: "light" },
    dark: { ...page, colorScheme: "dark" },
    phone: { device: "iPhone 15" },
  },
  run: ["light", "dark"],
  // The link signs the browser in. Every pass has a browser of its own.
  setup: async (page) => {
    await page.goto(link.href);
  },
  // A page that never opened is said, not pictured.
  failWhen: [
    { selector: "text=Not signed in", message: "the try's link didn't sign in: is the try this checkout's, and up? (scripts/try.sh info)" },
    { selector: "text=Opening the workspace…", message: "the page never opened the workspace" },
  ],
  exports: {
    // WebP: a fifth of the PNG's size, and the words as sharp.
    readme: {
      dir: process.env.SHOTS_OUT ?? "../docs/assets/shots",
      quality: 82,
      shots: pictures({
        mission: "pictures/mission",
        missions: "pictures/missions",
        review: "pictures/review",
        verify: "pictures/verify",
      }),
    },
    features: {
      dir: process.env.SHOTS_OUT ?? "../docs/assets/shots",
      quality: 82,
      shots: pictures({
        brief: "pictures/brief",
        work: "pictures/work",
        reviews: "pictures/reviews",
        "pull-request": "pictures/pull-request",
        documents: "pictures/documents",
        space: "pictures/space",
        document: "pictures/document",
        artifact: "pictures/artifact",
        conversation: "pictures/conversation",
        files: "pictures/files",
        team: "pictures/team",
        settings: "pictures/settings",
        orchestrator: "pictures/orchestrator",
        find: "find/find",
      }),
    },
  },
});
