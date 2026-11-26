import { ApiError } from "../../api";
import { humanError } from "../../ui/strings";

// Failed is what didn't work, to say above the list: a sentence, and what
// a build printed on the way, where it got that far.
export interface Failed {
  what: string;
  error: string;
  output?: string;
}

export const failed = (what: string, e: unknown): Failed => ({
  what,
  error: humanError((e as Error).message),
  output: e instanceof ApiError ? e.output : undefined,
});
