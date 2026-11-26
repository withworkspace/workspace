import type { CrewDetail } from "../api";
import { exitWords } from "../ui/statusOf";

// endedWords is how crew ended, in words: stopped by a restart, closed once
// it handed its work back, stopped by you, or its exit's words.
export function endedWords(c: Pick<CrewDetail, "how" | "exit">): string | undefined {
  switch (c.how) {
    case "interrupted":
      return "stopped by a Workspace restart";
    case "handed-back":
      return "closed: work handed back";
    case "stopped":
      return "stopped";
  }
  return exitWords(c.exit);
}
