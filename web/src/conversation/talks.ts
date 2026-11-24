import { create } from "zustand";
import { ApiError, getTalk, keepAsk, sayInTalk, settleTalkProposal, type TalkDetail, type TalkGiven } from "../api";
import { bursts } from "../load";
import { pageSocket } from "../socket";
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
// (conversation:<scope>/<id>): read again when a turn's said in it, or its
// agent has written more of its steps as it answers. Nothing reads it
// again and again.

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
    const leave = pageSocket.follow(talkTopic(at.slug, at.id), { following: () => void get().load(to), changed: changes.changed });
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
    told: (to, d) => put(to, { detail: d }),
    load: (to) => {
      const at = where(to);
      if (!at) return Promise.resolve();
      return getTalk(at.slug, at.id).then(
        (d) => get().told(to, d),
        (e: Error) => failed(to, e),
      );
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
