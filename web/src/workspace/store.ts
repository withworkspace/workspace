import { useEffect } from "react";
import { create } from "zustand";
import { type CommandLabel, type Connection, type FocusPin, getJSON, type Plugin, useEvents, type Workspace } from "../api";
import { keepSame } from "./keepSame";

// The workspace as the daemon says it is, in one store: the workspace
// itself (its missions, conversations, spaces, what needs you), its
// plugins, and what its commands are called. It's read whole when the
// daemon says it's changed, by the one thing that follows it
// (useFollowWorkspace, in the shell), and anything in the window reads the
// part it needs from here: nothing's handed down.
//
// What's read is kept the same where it says what it said before
// (keepSame), so a part that hasn't changed is the object it was, and
// what reads only that part isn't drawn again: a crew's activity moving
// doesn't draw the documents.

interface WorkspaceStore {
  // Undefined until it's first read.
  workspace?: Workspace;
  // Why it couldn't be read, the last time it was asked for.
  error?: Error;
  // How the stream that follows it is.
  connection: Connection;
  // Undefined until they're first read; none it can show is none.
  plugins?: Plugin[];
  // What ws's commands are doing, as an agent's steps show them ("Looking
  // up sketch-5"): ws's own and its plugins'. [] until it's read, when
  // steps show their commands.
  commandLabels: CommandLabel[];
  // Your pins as a write of them answered, until the workspace is next
  // read (shell/focus.tsx): they're the workspace's from then.
  focusAnswered?: FocusPin[];

  // read reads the workspace again; readPlugins its plugins, and with
  // them what the commands are called (a plugin's are as it last started).
  read: () => Promise<void>;
  readPlugins: () => Promise<void>;
  readCommandLabels: () => Promise<void>;
}

let commandLabelsAsked = false;

export const useWorkspaceStore = create<WorkspaceStore>()((set, get) => ({
  connection: "connecting",
  commandLabels: [],

  read: () =>
    getJSON<Workspace>("/api/workspace").then(
      (workspace) => set((s) => ({ workspace: keepSame(s.workspace, workspace), error: undefined, focusAnswered: undefined })),
      (error: Error) => set({ error }),
    ),
  readPlugins: () =>
    getJSON<Plugin[]>("/api/plugins").then(
      (plugins) => {
        void get().readCommandLabels();
        set((s) => ({ plugins: keepSame(s.plugins, plugins) }));
      },
      () => set((s) => ({ plugins: s.plugins ?? [] })),
    ),
  readCommandLabels: () => {
    commandLabelsAsked = true;
    return getJSON<CommandLabel[]>("/api/commands").then(
      (labels) => set((s) => ({ commandLabels: keepSame(s.commandLabels, labels) })),
      () => {}, // steps show their commands
    );
  },
}));

// useFollowWorkspace keeps the store current, for as long as it's mounted
// (once, in the shell) and the page is showing: it reads the workspace
// when it's followed on the tab's socket (so nothing that changed while it was closed
// is missed) and again whenever the daemon says it changed. It never
// polls. The plugins are followed the same way; showing is the plugin
// whose app the page shows, if it shows one. That stream is how ws knows
// a page is showing, and with showing, which plugin's app is on screen:
// it tells plugins who's looking, and they fetch to suit.
export function useFollowWorkspace(showing?: string) {
  const { read, readPlugins } = useWorkspaceStore.getState();
  const visible = useEvents("workspace", {
    hello: () => {
      useWorkspaceStore.setState({ connection: "live" });
      void read();
    },
    changed: () => void read(),
    lost: (closed) => {
      useWorkspaceStore.setState({ connection: "lost" });
      // A stream refused for being signed out never opens: find out why.
      if (closed) void read();
    },
  });
  useEffect(() => {
    if (visible) useWorkspaceStore.setState({ connection: "connecting" });
  }, [visible]);
  useEvents("plugins", { hello: () => void readPlugins(), changed: () => void readPlugins() }, showing ? `&app=${encodeURIComponent(showing)}` : "");
}

// useWorkspace is the workspace, for a page: there once it's been read,
// which is before any page is drawn. With pick, it's that part of it, and
// what asks is drawn again only when that part changes. pick gives
// something of the workspace as it is (w.missions, w.root), not something
// made from it (a filtered list is a new one each time, and never the
// same): make that from the part, in a memo.
export function useWorkspace(): Workspace;
export function useWorkspace<T>(pick: (w: Workspace) => T): T;
export function useWorkspace<T>(pick?: (w: Workspace) => T): T | Workspace {
  const read = useWorkspaceStore((s) => s.workspace !== undefined);
  if (!read) throw new Error("The workspace hasn't been read yet: this is drawn before it could be.");
  return useWorkspaceStore((s) => (pick ? pick(s.workspace!) : s.workspace!));
}

// useWorkspaceIfRead is the workspace for what's drawn before it's read
// (the shell): undefined until then. With pick, that part of it.
export function useWorkspaceIfRead(): Workspace | undefined;
export function useWorkspaceIfRead<T>(pick: (w: Workspace) => T): T | undefined;
export function useWorkspaceIfRead<T>(pick?: (w: Workspace) => T): T | Workspace | undefined {
  return useWorkspaceStore((s) => (s.workspace && pick ? pick(s.workspace) : s.workspace));
}

// usePlugins are the workspace's plugins; undefined until they're first
// read.
export const usePlugins = (): Plugin[] | undefined => useWorkspaceStore((s) => s.plugins);

// useCommandLabels is what ws's commands are called. Read once for the
// page, and again as the plugins change.
export function useCommandLabels(): CommandLabel[] {
  useEffect(() => {
    if (!commandLabelsAsked) void useWorkspaceStore.getState().readCommandLabels();
  }, []);
  return useWorkspaceStore((s) => s.commandLabels);
}
