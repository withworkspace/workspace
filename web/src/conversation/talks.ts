import { create } from "zustand";
import { ApiError, getTalk, keepAsk, sayInTalk, settleTalkProposal, type TalkDetail, type TalkGiven, type TalkTurn } from "../api";
import { bursts } from "../load";
import { pageSocket } from "../socket";
import type { ConversationOp } from "../wire.gen";
import { paneAt } from "../chrome/panes";

// The conversations the window is holding, in one store, by address (a
// conversation's own page's: chrome/panes.ts's talkAt). What a bar asks
// lands here; the pane beside the page draws it from here, as its own
// page does, and a mission's Brief tab its planner's; the bar that asked
// reads from here whether it's still being answered, or was kept. So a
// conversation isn't any one component's: it outlives the page that
// asked it, and two places showing one show the same. Come back to one
// and it's there as it was last read, while it's read again.
//
// One that something's showing (watch) is followed, over the tab's socket
// (conversation:<scope>/<id>): read whole once it's followed, and from
// then on told how it changes (a turn said, its agent's steps as it
// answers, its status), which is applied to it as held (apply). It's read
// whole again only when a change can't be applied: one's been missed, or
// the server couldn't say. Nothing reads it again and again.

export interface Held {
  detail?: TalkDetail;
  // It couldn't be read, and why: said only while there's nothing of it
  // to show, or it's gone.
  error?: string;
  // It's gone: an ask nobody kept, swept an hour after its last turn.
  gone?: boolean;
}

interface TalksStore {
  talks: Record<string, Held>;
  // told is a conversation as it was just read or answered somewhere.
  told: (to: string, d: TalkDetail) => void;
  // apply is a change told over the socket, applied to one as it's held.
  apply: (to: string, op: ConversationOp) => void;
  // load reads one again.
  load: (to: string) => Promise<void>;
  // say says something in one; keep keeps an ask as a conversation;
  // settle accepts (with your edits) or declines something it proposed.
  // Each rejects with why it couldn't, having noted one that's gone.
  say: (to: string, body: string, given?: TalkGiven) => Promise<TalkDetail>;
  keep: (to: string) => Promise<TalkDetail>;
  settle: (to: string, n: number, action: "accept" | "decline", edits?: object) => Promise<TalkDetail>;
  // watch is something showing one, until what's returned is called:
  // while it's watched, it's followed.
  watch: (to: string) => () => void;
  forget: (to: string) => void;
}

// watched is how many things show each conversation, and following how to
// stop following each.
const watched = new Map<string, number>();
const following = new Map<string, () => void>();
// loading is each conversation being read now.
const loading = new Map<string, Promise<void>>();

// talkTopic is a conversation's topic: its scope (its mission's slug, or
// nothing for the workspace's) and its id, as the records name it.
export const talkTopic = (slug: string, id: string) => `conversation:${slug}/${id}` as const;

const where = (to: string) => {
  const at = paneAt(to);
  return at?.kind === "conversation" ? at : undefined;
};

export const useTalks = create<TalksStore>()((set, get) => {
  const put = (to: string, held: Held) => set((s) => ({ talks: { ...s.talks, [to]: held } }));
  // What comes of asking the daemon about one: it's held as answered, or
  // noted as gone, or as not read.
  const failed = (to: string, e: Error) => {
    if (e instanceof ApiError && e.status === 404) put(to, { detail: get().talks[to]?.detail, gone: true, error: e.message });
    else if (!get().talks[to]?.detail) put(to, { error: e.message });
  };
  // Following one: it's read once it's followed (so nothing said before
  // is missed) and on each change, those within a moment of each other
  // once (bursts).
  const follow = (to: string) => {
    const at = where(to);
    if (!at || following.has(to)) return;
    const changes = bursts(() => void get().load(to));
    const leave = pageSocket.follow(talkTopic(at.slug, at.id), {
      following: () => void get().load(to),
      changed: (said) => (said.conversation ? get().apply(to, said.conversation) : changes.changed()),
    });
    following.set(to, () => {
      changes.stop();
      leave();
    });
  };
  const act = (to: string, ask: (slug: string, id: string) => Promise<TalkDetail>) => {
    const at = where(to);
    if (!at) return Promise.reject(new Error("That isn't a conversation."));
    return ask(at.slug, at.id).then(
      (d) => {
        get().told(to, d);
        return d;
      },
      (e: Error) => {
        failed(to, e);
        throw e;
      },
    );
  };
  return {
    talks: {},
    told: (to, d) => {
      // A read that crossed a change told after it is older than what's held.
      const held = get().talks[to]?.detail;
      if (held?.version !== undefined && d.version !== undefined && d.version < held.version) return;
      put(to, { detail: d });
    },
    apply: (to, op) => {
      const next = applied(get().talks[to]?.detail, op);
      if (next === "read") void get().load(to);
      else if (next) put(to, { detail: next });
    },
    load: (to) => {
      const at = where(to);
      if (!at) return Promise.resolve();
      // Asked again while it's being read (a page opening, as it's
      // followed): that read answers both.
      const reading = loading.get(to);
      if (reading) return reading;
      const read = getTalk(at.slug, at.id)
        .then(
          (d) => get().told(to, d),
          (e: Error) => failed(to, e),
        )
        .finally(() => loading.delete(to));
      loading.set(to, read);
      return read;
    },
    say: (to, body, given) => act(to, (slug, id) => sayInTalk(slug, id, body, given)),
    keep: (to) => act(to, (slug, id) => keepAsk(slug, id)),
    settle: (to, n, action, edits) => act(to, (slug, id) => settleTalkProposal(slug, id, n, action, edits)),
    watch: (to) => {
      watched.set(to, (watched.get(to) ?? 0) + 1);
      follow(to);
      return () => {
        const n = (watched.get(to) ?? 1) - 1;
        if (n > 0) {
          watched.set(to, n);
          return;
        }
        watched.delete(to);
        following.get(to)?.();
        following.delete(to);
      };
    },
    forget: (to) =>
      set((s) => {
        if (!(to in s.talks)) return s;
        const { [to]: _, ...rest } = s.talks;
        return { talks: rest };
      }),
  };
});

// useTalk is a conversation as the window holds it.
export const useTalk = (to: string | undefined): Held | undefined => useTalks((s) => (to ? s.talks[to] : undefined));

// talkName is what a conversation's called, for its tab: its title, or
// what it opened with.
export const talkName = (d: TalkDetail | undefined): string | undefined => d && (d.title || d.turns[0]?.body.slice(0, 80) || undefined);

// applied is a conversation held with a change told over the socket
// applied: undefined when the change is in it already (held as of a later
// version), and "read" when it can't be applied (nothing's held, or turns
// it should have are missing): it's read whole then.
export function applied(held: TalkDetail | undefined, op: ConversationOp): TalkDetail | undefined | "read" {
  if (!held || !op.conversation) return "read";
  if (held.version !== undefined && op.version < held.version) return undefined;
  if (op.from > held.turns.length) return "read";
  // The change is the server's conversation, which the web UI's own type
  // says more narrowly (its statuses, its proposals' kinds): the same JSON.
  const rest = op.conversation as unknown as Omit<TalkDetail, "turns" | "run" | "version">;
  return { ...rest, turns: [...held.turns.slice(0, op.from), ...(op.turns as unknown as TalkTurn[])], run: op.run, version: op.version };
}
