import { ApiError, type PR } from "../api";
import { useLoaded } from "../load";
import { prOf } from "./prs";

// IssueIs is what GitHub says an issue's number is, as a pull request's
// card asks it (prs.ts): an open pull request; an issue, or a pull request
// that's no longer open, which that question can't tell apart; or unknown,
// where GitHub couldn't be asked. Asking, until it answers.
export type IssueIs = { is: "pr"; pr: PR } | { is: "issue" | "unknown" | "asking" };

// issueIs asks: a number that isn't an open pull request is a 404.
export const issueIs = (repo: string, number: number): Promise<IssueIs> =>
  prOf(repo, number).then(
    (pr): IssueIs => ({ is: "pr", pr }),
    (e): IssueIs => ({ is: e instanceof ApiError && e.status === 404 ? "issue" : "unknown" }),
  );

// useIssueIs asks what an issue's number is, unless it's been said
// (said, as a story says it): nothing's asked then.
export function useIssueIs(repo: string, number: number, said?: IssueIs): IssueIs {
  const { value } = useLoaded(said ? null : () => issueIs(repo, number), [repo, number, !!said]);
  return said ?? value ?? { is: "asking" };
}
