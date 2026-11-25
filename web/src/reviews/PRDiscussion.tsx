import type { Discussion } from "../api";
import { humanError } from "../ui/text";
import { SaidHead } from "./SaidHead";
import { SaidItem } from "./SaidItem";
import { SaidList } from "./SaidList";
import { SayBox } from "./SayBox";

// A review's overview and its discussion are views beside its changes. A
// pull request's overview is its description, as its author wrote it; a
// capsule's is its mission. The discussion is what's been said on the
// pull request itself on GitHub (reviews and comments; comments on lines
// are in the changes).

// PRDiscussion is what's been said on the pull request itself, and a box
// to say something.
export function PRDiscussion({
  api,
  discussion,
  error,
  onSaid,
}: {
  api: string;
  discussion?: Discussion;
  error?: string; // why what's on GitHub couldn't be read
  onSaid: (d: Discussion) => void;
}) {
  return (
    <>
      <h2>On GitHub</h2>
      {error ? (
        <p className="dim">What's been said on GitHub couldn't be read: {humanError(error)}</p>
      ) : !discussion ? (
        <p className="dim">Reading what's been said…</p>
      ) : (
        <>
          {discussion.items.length === 0 && <p className="dim">Nothing's been said on the pull request itself yet. Comments on lines are in the changes.</p>}
          <SaidList>
            {discussion.items.map((it) => (
              <SaidItem key={it.id} head={<SaidHead it={it} />} body={it.body} bot={it.bot} />
            ))}
          </SaidList>
          <SayBox api={api} onSaid={onSaid} />
        </>
      )}
    </>
  );
}
