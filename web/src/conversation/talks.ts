import { create } from "zustand";
import { ApiError, getTalk, keepAsk, sayInTalk, settleTalkProposal, type TalkDetail, type TalkGiven } from "../api";
import { waiting } from "../ask/ask";
import { paneAt } from "../chrome/panes";

// The conversations the window is holding, in one store, by address (a
// conversation's own page's: chrome/panes.ts's talkAt). What a bar asks
// lands here; the pane beside the page draws it from here; the bar that
// asked reads from here whether it's still being answered, or was kept.
// So a conversation isn't any one component's: it outlives the page that
// asked it, and two places showing one show the same.
//
// One being answered is followed (read again every second) for as long
// as something is showing it (watch).

export interface Held {
  detail?: TalkDetail;
  // It couldn't be read, and why.
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
  // while it's watched and being answered, it's followed.
  watch: (to: string) => () => void;
  forget: (to: string) => void;
}

// How often one's read again while its agent's answering.
const followEvery = 1000;

const watched = new Map<string, number>();
let timer: ReturnType<typeof setTimeout> | undefined;

const where = (to: string) => {
  const at = paneAt(to);
  return at?.kind === "conversation" ? at : undefined;
};

export const useTalks = create<TalksStore>()((set, get) => {
  const put = (to: string, held: Held) => set((s) => ({ talks: { ...s.talks, [to]: held } }));
  // What comes of asking the daemon about one: it's held as answered, or
  // noted as gone, or as not read.
  const failed = (to: string, e: Error) => {
    if (e instanceof ApiError && e.status === 404) put(to, { detail: get().talks[to]?.detail, gone: true });
    else if (!get().talks[to]?.detail) put(to, { error: e.message });
  };
  // Following: while anything watched is being answered, read those
  // again in a moment, and look again after.
  const follow = () => {
    if (timer) return;
    const due = [...watched.keys()].filter((to) => waiting(get().talks[to]?.detail));
    if (!due.length) return;
    timer = setTimeout(() => {
      timer = undefined;
      void Promise.all(due.map((to) => get().load(to))).then(follow);
    }, followEvery);
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
      put(to, { detail: d });
      follow();
    },
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
      follow();
      return () => {
        const n = (watched.get(to) ?? 1) - 1;
        if (n > 0) watched.set(to, n);
        else watched.delete(to);
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
