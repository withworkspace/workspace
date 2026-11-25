import type { Meta, StoryObj } from "@storybook/react-vite";
import type { WhatsNewEntry, WhatsNewInfo } from "../../api";
import taken from "./fixtures/whats-new.json";
import { News } from "./News";

// What's new, as it opens over a page: drawn from what a try's ws said
// (fixtures/whats-new.json, GET /api/whats-new on a build of this repo),
// its pictures the entries' own (Storybook serves internal/whatsnew/entries
// where the daemon would). A dialog covers the page, so each state is a
// story of its own: one entry, with a picture and without; several, the
// first with a picture and without; a build of a clone; nothing; and more
// than this ws keeps.

const meta: Meta = { title: "Shell/WhatsNew" };
export default meta;

const as = taken as WhatsNewInfo;
const entry = (slug: string): WhatsNewEntry => as.entries.find((e) => e.slug === slug)!;
// After an upgrade from v0.12: these are unseen, and it says since when.
const upgraded = (unseen: WhatsNewEntry[], more?: Partial<WhatsNewInfo>): WhatsNewInfo => ({ ...as, version: "v0.17.0", release: true, seen: "v0.12.0", unseen, ...more });
const open = (info: WhatsNewInfo, entries = info.unseen) => ({
  render: () => <News title="What's new" entries={entries} info={info} onClose={() => {}} />,
});

export const OneEntry: StoryObj = open(upgraded([entry("ask-beside")]));

export const OneWithAPicture: StoryObj = open(upgraded([entry("the-foot")]));

export const Several: StoryObj = open(upgraded(as.entries));

export const SeveralLedByAPicture: StoryObj = open(upgraded([entry("the-foot"), entry("ask-beside"), entry("plugins-in-settings"), entry("silos-kept-by-the-daemon")]));

export const OneRelease: StoryObj = open(upgraded([entry("plugins-in-settings"), entry("find-acts-here"), entry("whats-new")]));

export const FromFurtherBack: StoryObj = open(upgraded([entry("ask-beside"), entry("the-foot")], { missing: true, releases: "https://github.com/withworkspace/workspace/releases" }));

export const BuildOfAClone: StoryObj = open(as, as.entries);

export const Nothing: StoryObj = open({ ...as, release: true, entries: [] }, []);
