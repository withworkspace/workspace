import { describe, expect, it } from "vitest";
import { keptBefore, migratePrefs, setPref, usePrefs } from "./prefs";

const storage = (kept: Record<string, string>) => ({ getItem: (k: string) => kept[k] ?? null });

describe("what was kept before the prefs were one entry", () => {
  it("is read as it was written, each its own way", () => {
    expect(
      keptBefore(
        storage({
          "ws-notify": "on",
          "ws-file-panes": "2",
          "ws:mission-side": "0",
          "ws:work-done": "1",
          "ws:brief-planner": "0",
          "ws:files-w": "320",
          "ws:split": "1",
          "ws:guide-order": "0",
          "ws:pass-hide": '{"security":true}',
          "ws:page-map": "off",
          "ws:pass-notes": '{"style":false}',
          "ws:space-list-w": "400",
          "ws:talk-side": "off",
          "ws.artifacts.found.folded": '["api:docs"]',
          "ws-dismissed": '["warning: this sign-in ends"]',
        }),
      ),
    ).toEqual({
      notify: true,
      filePanes: 2,
      missionSide: false,
      workDone: true,
      briefPlanner: false,
      reviewFilesWidth: 320,
      reviewSplit: true,
      guideOrder: false,
      passHide: { security: true },
      pageMap: false,
      passNotes: { style: false },
      spaceListWidth: 400,
      talkSide: false,
      foundFolded: ["api:docs"],
      dismissed: ["warning: this sign-in ends"],
    });
  });

  it("is nothing where nothing was kept, or what was isn't what it should be", () => {
    expect(keptBefore(storage({}))).toEqual({});
    expect(keptBefore(storage({ "ws-file-panes": "7", "ws:files-w": "wide", "ws:pass-hide": "[1]", "ws-dismissed": "{" }))).toEqual({});
  });
});

describe("a pref", () => {
  it("is set to a value, or to what's made of how it was", () => {
    setPref("reviewSplit", true);
    expect(usePrefs.getState().reviewSplit).toBe(true);
    setPref("reviewSplit", (was) => !was);
    expect(usePrefs.getState().reviewSplit).toBe(false);
    setPref("passHide", (was) => ({ ...was, style: true }));
    expect(usePrefs.getState().passHide).toEqual({ style: true });
    setPref("passHide", {});
  });
});

describe("prefs kept by an earlier ws", () => {
  it("have desktop notifications on: they were off by default, and nothing said so", () => {
    const kept = { ...usePrefs.getState(), notify: false };
    expect(migratePrefs(kept, 1).notify).toBe(true);
    expect(migratePrefs(kept, 2).notify).toBe(false); // turned off since: left off
    expect(usePrefs.getState().notify).toBe(true); // and on for a browser that kept nothing
  });
});
