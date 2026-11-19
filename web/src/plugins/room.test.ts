import { describe, expect, it } from "vitest";
import { frameRoom, setFrameRoom } from "./room";

describe("a plugin's page that wants the room", () => {
  it("has it until its page says it doesn't, or goes", () => {
    setFrameRoom("excalidraw", true);
    expect(frameRoom()).toBe("excalidraw");
    setFrameRoom("excalidraw", false);
    expect(frameRoom()).toBeUndefined();
  });

  it("keeps it as another plugin's page goes", () => {
    setFrameRoom("excalidraw", true);
    setFrameRoom("jira", false);
    expect(frameRoom()).toBe("excalidraw");
    setFrameRoom("excalidraw", false);
  });
});
