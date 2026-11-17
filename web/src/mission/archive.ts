import { act, ApiError } from "../api";
import { confirm, humanError } from "../ui/kit";

// archiveMission archives a mission, which burns its capsules. When one's
// work hasn't merged, the server refuses, saying which and why: you're
// asked, and agreeing archives it again, burning them anyway. Undefined
// is no, nothing done.
export async function archiveMission(slug: string): Promise<Awaited<ReturnType<typeof act>> | undefined> {
  try {
    return await act(slug, "archive");
  } catch (e) {
    if (!(e instanceof ApiError) || e.code !== "unmerged-capsules") throw e;
    const yes = await confirm({ title: "Archive and burn its capsules?", body: humanError(e.message), confirm: "Archive and burn", danger: true });
    return yes ? act(slug, "archive", { burnAll: true }) : undefined;
  }
}
