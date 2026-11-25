import type { Talk } from "../api";
import { Inline } from "../ui/Inline";
import { type PageRef, refHref } from "./refs";
import { RefCardHead } from "./RefCardHead";
import { RefGo } from "./RefGo";

// RefTalkCard is a conversation's card: what it's called, and what it came
// to.
export function RefTalkCard({ r, talk: t }: { r: Extract<PageRef, { kind: "talk" }>; talk?: Talk }) {
  return (
    <>
      <RefCardHead icon="message" title={t?.title || t?.opening || r.id} />
      {t?.summary && (
        <span className="dim">
          <Inline text={t.summary} lines={4} />
        </span>
      )}
      <RefGo to={refHref(r)}>Open the conversation</RefGo>
    </>
  );
}
