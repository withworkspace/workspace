import { Chip } from "../../ui/Chip";
import { HoverCardHead, HoverCardLine, HoverCardTitle } from "../../ui/HoverCard";
import { Icon } from "../../ui/Icon";
import type { PullFound } from "../find";
import { textClass } from "../../ui/Text";

// FindPullCard says of a pull request, in Find's pane, what Find was told
// of it: whose it is, where it stands, its checks, its branch and its
// stack. It doesn't ask GitHub: looking down a list shouldn't. One that
// isn't among the open pull requests here (pull.asks) says choosing it
// will.
export function FindPullCard({ pull: p }: { pull: PullFound }) {
  return (
    <>
      <HoverCardHead>
        <Icon name="pr" size={13} className={textClass("dim")} />
        <HoverCardTitle>{p.asks ? `${p.repo}#${p.number}` : p.title}</HoverCardTitle>
      </HoverCardHead>
      {p.asks ? (
        <HoverCardLine dim>It isn't among the open pull requests here. Choosing it asks GitHub.</HoverCardLine>
      ) : (
        <>
          <HoverCardLine dim>
            {p.repo}#{p.number}
            {p.author ? `, by ${p.author}` : ""}
            {p.draft ? ", a draft" : ""}
          </HoverCardLine>
          {p.words && (
            <HoverCardLine>
              {p.words}
              {p.checks && (
                <Chip tone={p.checks === "success" ? "done" : p.checks === "failure" ? "danger" : "quiet"}>
                  checks {p.checks === "success" ? "pass" : p.checks === "failure" ? "fail" : "running"}
                </Chip>
              )}
            </HoverCardLine>
          )}
          {p.branch && (
            <HoverCardLine dim className={textClass("mono")}>
              {p.branch}
            </HoverCardLine>
          )}
          {p.stack && <HoverCardLine dim>In a stack: {p.stack}</HoverCardLine>}
        </>
      )}
    </>
  );
}
