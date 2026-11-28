import type { Meta, StoryObj } from "@storybook/react-vite";
import { MemoryRouter } from "react-router";
import type { Lesson } from "../api";
import { Button } from "../ui/Button";
import { Empty } from "../ui/Empty";
import { GroupHead } from "../ui/GroupHead";
import { PageHead } from "../ui/PageHead";
import { Rows } from "../ui/Rows";
import { LessonCard } from "./LessonCard";
import { LessonSightings } from "./LessonSightings";
import { LessonsFoot, LessonsFrame, LessonsLede, LessonsSection } from "./LessonsFrame";
import { lessonGroups, readingWords } from "./lessonWords";

// The Lessons page, from its parts: a recommendation for each fix, one
// crew ran into after it was fixed, a note, one nobody's run into lately,
// fixed and dismissed ones; what crew said, opened; and the page empty.

const meta: Meta = {
  title: "Lessons/Lessons",
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
};
export default meta;

const ago = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString();

const lesson = (id: number, more: Partial<Lesson>): Lesson => ({
  id,
  title: "",
  fix: "setup",
  change: "",
  why: "",
  state: "open",
  stateAt: ago(24 * 9),
  created: ago(24 * 9),
  seen: [],
  crew: 1,
  missions: 1,
  lastSeen: ago(5),
  ...more,
});

const webDeps = lesson(1, {
  title: "Web dependencies missing in a fresh capsule",
  repo: "web",
  change: 'Set `after_create = "go mod download && (cd web && pnpm install --frozen-lockfile)"` under `[repos.web]` in ws.toml.',
  why: "after_create installs Go's modules only, so a new capsule has no web/node_modules.",
  crew: 3,
  missions: 2,
  seen: [
    { mission: "invoice-pdf-totals", crew: "c2", repo: "web", said: "`pnpm test` failed with `vitest: command not found` until `pnpm install` in web/.", at: ago(5) },
    { mission: "invoice-pdf-totals", crew: "c1", repo: "web", said: "A fresh capsule has no web/node_modules: `tsc` isn't found.", at: ago(30) },
    { mission: "studio-onboarding", crew: "c3", repo: "web", said: "Ran `pnpm install --frozen-lockfile` before the type-check could run.", at: ago(24 * 6) },
  ],
});

const snapshots = lesson(2, {
  title: "PDF snapshot tests need wkhtmltopdf",
  repo: "api",
  fix: "you",
  change: "Install wkhtmltopdf on this machine (`brew install wkhtmltopdf`), or the PDF snapshot tests fail in every capsule.",
  why: "`just test` runs the PDF snapshot tests, which call wkhtmltopdf.",
  crew: 2,
  missions: 2,
  seen: [
    { mission: "tax-rounding", crew: "c1", repo: "api", said: "`just test` fails on the PDF snapshots: wkhtmltopdf isn't installed. `go test ./internal/...` runs the rest.", at: ago(26) },
    { mission: "late-fee-reminders", crew: "c2", repo: "api", said: "PDF snapshot tests fail: no wkhtmltopdf.", at: ago(24 * 4) },
  ],
});

const tz = lesson(3, {
  title: "Date tests fail outside UTC",
  repo: "api",
  fix: "repo",
  change: "Set `TZ=UTC` in the `test` recipe in the justfile, or make `TestDueDates` set the zone itself.",
  why: "TestDueDates reads the machine's zone.",
  crew: 2,
  missions: 1,
  again: ago(8),
  stateAt: ago(8),
  seen: [
    { mission: "late-fee-reminders", crew: "c3", repo: "api", said: "TestDueDates failed until `TZ=UTC just test`.", at: ago(8) },
    { mission: "late-fee-reminders", crew: "c1", repo: "api", said: "Due dates are a day out unless TZ=UTC.", at: ago(24 * 5) },
  ],
});

const note = lesson(4, {
  title: "Seed data needs the local database up",
  repo: "api",
  fix: "docs",
  change: "Say in AGENTS.md that `just db-up` comes before `just seed`.",
  seen: [{ mission: "studio-onboarding", crew: "c2", repo: "api", said: "`just seed` hung until `just db-up` had started Postgres.", at: ago(50) }],
});

const quiet = lesson(5, {
  title: "Browser checks need a cached Chromium",
  fix: "you",
  change: "Run `pnpm exec playwright install chromium-headless-shell` once on this machine.",
  crew: 2,
  missions: 2,
  lastSeen: ago(24 * 40),
  seen: [
    { mission: "invoice-pdf-totals", crew: "c4", said: "Playwright had no browser; installed chromium-headless-shell.", at: ago(24 * 40) },
    { mission: "studio-onboarding", crew: "c1", said: "No Chromium for the browser check.", at: ago(24 * 44) },
  ],
});

const fixed = lesson(6, {
  title: "Lint fails on generated files",
  repo: "web",
  fix: "repo",
  change: "Leave `src/generated` out in `eslint.config.js`.",
  state: "fixed",
  crew: 2,
  missions: 2,
  mission: "lint-skips-generated-files",
  lastSeen: ago(24 * 7),
});

const dismissed = lesson(7, {
  title: "npm audit warns in dev dependencies",
  repo: "web",
  fix: "docs",
  change: "Nothing to change: the warnings are in dev dependencies only.",
  state: "dismissed",
  lastSeen: ago(24 * 3),
});

const all = [webDeps, snapshots, tz, note, quiet, fixed, dismissed];

const actions = (l: Lesson) =>
  l.state !== "open" ? (
    <Button size="small" variant="ghost">
      Reopen
    </Button>
  ) : (
    <>
      <Button size="small">Start mission</Button>
      <Button size="small" variant="ghost">
        Mark fixed
      </Button>
      <Button size="small" variant="ghost">
        Dismiss
      </Button>
    </>
  );

export const Page: StoryObj = {
  render: () => (
    <LessonsFrame>
      <PageHead title="Lessons" />
      <LessonsLede>What crew keep running into where they work, from their handoffs, and what would stop it.</LessonsLede>
      {lessonGroups(all, Date.now()).map((g) => (
        <LessonsSection key={g.id}>
          <GroupHead count={g.lessons.length}>{g.title}</GroupHead>
          <Rows>
            {g.lessons.map((l) => (
              <LessonCard key={l.id} lesson={l} actions={actions(l)} />
            ))}
          </Rows>
        </LessonsSection>
      ))}
      <LessonsFoot>{readingWords({ on: true, read: 31, failed: 1, waiting: 4 })}</LessonsFoot>
    </LessonsFrame>
  ),
};

export const Opened: StoryObj = {
  render: () => (
    <Rows>
      <LessonCard lesson={webDeps} actions={actions(webDeps)} sightingsOpen flash />
    </Rows>
  ),
};

export const Sightings: StoryObj = {
  render: () => <LessonSightings seen={webDeps.seen} open />,
};

export const Nothing: StoryObj = {
  render: () => (
    <LessonsFrame>
      <PageHead title="Lessons" />
      <Empty icon="tool" title="No lessons yet">
        Each crew notes what got in its way under Environment in its handoff. When two crew run into the same thing, it
        shows here with what would fix it.
      </Empty>
      <LessonsFoot>{readingWords({ on: false, read: 0, failed: 0, waiting: 3 })}</LessonsFoot>
    </LessonsFrame>
  ),
};
