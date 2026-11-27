import { type Ref, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { Crew } from "../../api";
import { pageScroller } from "../../ui/scroller";
import { useWorkspace } from "../../workspace/store";
import { CrewConversationView } from "./CrewConversationView";
import { useFileLink } from "./useFileLink";
import { useTranscript } from "./useTranscript";

// CrewConversation is a crew as a conversation, the way in to it: what it
// was sent, what it said and the steps between, as it happens (the
// session file, followed: useTranscript); its plan pinned on top; what
// it's asking (a permission, a question) answered from here; and a box to
// tell it what's next, or to resume it with, once it's ended. It opens on
// the latest, then follows it, unless you've scrolled up to read, when a
// pill says there's newer. CrewConversationView draws it.
export function CrewConversation({ slug, id, crew, live, onTerminal, field }: { slug: string; id: string; crew?: Crew; live: boolean; onTerminal?: () => void; field?: Ref<HTMLElement> }) {
  const workspace = useWorkspace();
  const { t, error, reload } = useTranscript(slug, id);
  const [unseen, setUnseen] = useState(false);
  const follow = useRef(true);
  const first = useRef(true);
  const fileLink = useFileLink(workspace, slug, crew);

  useEffect(() => {
    const m = pageScroller();
    if (!m) return;
    const onScroll = () => {
      follow.current = m.scrollHeight - m.scrollTop - m.clientHeight < 160;
      if (follow.current) setUnseen(false);
    };
    m.addEventListener("scroll", onScroll, { passive: true });
    return () => m.removeEventListener("scroll", onScroll);
  }, []);
  const size = (t?.earlier ?? 0) + (t?.entries.length ?? 0);
  const asking = live && (crew?.activity === "permission" || crew?.activity === "question");
  // Open on the latest; then follow it, unless you've scrolled up to read.
  useLayoutEffect(() => {
    if (!t) return;
    const m = pageScroller();
    if (!m) return;
    if (first.current || follow.current) {
      m.scrollTo({ top: m.scrollHeight, behavior: first.current ? "auto" : "smooth" });
      first.current = false;
    } else setUnseen(true);
  }, [size, t, asking]);

  const newer = () => {
    const m = pageScroller();
    m?.scrollTo({ top: m.scrollHeight, behavior: "smooth" });
    setUnseen(false);
  };
  return (
    <CrewConversationView
      slug={slug}
      id={id}
      crew={crew}
      live={live}
      t={t}
      error={error}
      reload={reload}
      root={workspace.root}
      fileLink={fileLink}
      onNewer={unseen ? newer : undefined}
      onTerminal={onTerminal}
      field={field}
    />
  );
}
