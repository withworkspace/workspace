import { mergePR, prAPI } from "../api";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";

// When a pull request's asked to merge: now, once it can, or no longer by
// itself (ops.MergePR).
export type MergeWhen = "now" | "ready" | "off";

// merge asks ws to merge a pull request (a stack whole, asked of its
// top), at the commit the page shows, and says in a toast what came of
// it or why it won't. It answers GitHub's status (merged, pending,
// enqueued; on or off), or nothing when it was refused.
export function merge(repo: string, number: number, when: MergeWhen, head?: string): Promise<string | undefined> {
  return mergePR(prAPI(repo, number), { when, head }).then(
    (r) => {
      toast(r.message, { tone: "done" });
      return r.status;
    },
    (e: Error) => {
      toast(humanError(e.message), { tone: "danger" });
      return undefined;
    },
  );
}
