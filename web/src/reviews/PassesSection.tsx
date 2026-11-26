import type { PassFinding, ReviewPass } from "../api";
import { PassList } from "./PassList";
import { PassView } from "./PassView";
import { Text } from "../ui/Text";

// A review's passes, a view of its own: each pass, go or no go, what it said
// of the change, and what it found, to take into your review or dismiss. Go
// or no go is worked out from what's still open, so it moves as you settle
// them.
export function PassesSection({
  api,
  passes,
  hidden,
  canAsk,
  toFinding,
  toComment,
  onResolve,
  onPass,
  onTook,
  onHide,
  onAsk,
}: {
  api: string;
  passes?: ReviewPass[];
  hidden: Record<string, boolean>; // passes whose findings aren't drawn in the diff
  canAsk: boolean;
  toFinding: (pass: string, key: string) => boolean; // whether it's in the diff to go to
  toComment: (id: string) => boolean;
  onResolve: (thread: string) => Promise<unknown>; // resolve the comment a follow-up's finding is about
  onPass: (p: ReviewPass) => void; // a pass as it now stands
  onTook: () => void; // a finding went into the draft
  onHide: (pass: string, hide: boolean) => void;
  onAsk: (pass: ReviewPass, f: PassFinding) => void;
}) {
  if (!passes) {
    return (
      <>
        <h2>Passes</h2>
        <Text as="p" dim>Loading passes…</Text>
      </>
    );
  }
  return (
    <>
      <h2>Passes</h2>
      {passes.length === 0 ? (
        <Text as="p" dim>
          No review passes set up. Add one under <Text mono>[reviews.passes.&lt;name&gt;]</Text> in{" "}
          <Text mono>ws.toml</Text> (team) or <Text mono>ws.local.toml</Text> (yours): a skill, or a few words
          on what to look for.
        </Text>
      ) : (
        <PassList>
          {passes.map((p) => (
            <PassView key={p.name} api={api} p={p} hidden={!!hidden[p.name]} canAsk={canAsk} toFinding={toFinding} toComment={toComment} onResolve={onResolve} onPass={onPass} onTook={onTook} onHide={onHide} onAsk={onAsk} />
          ))}
        </PassList>
      )}
    </>
  );
}
