import type { Posting } from "../api";
import { Icon } from "../ui/Icon";
import css from "./PullRequestPosting.module.css";
import { Text, textClass } from "../ui/Text";

// PullRequestPosting is exactly what will be posted on a pull request,
// and where: its verdict and commit, its body, each comment on its lines,
// and those whose lines GitHub hasn't, which go in the body.
export function PullRequestPosting({ p }: { p: Posting }) {
  return (
    <div className={css.posting}>
      <Text as="div" dim className={css.where}>
        Goes to{" "}
        <a href={p.url} target="_blank" rel="noreferrer" className={textClass("link")}>
          {p.url.replace(/^https:\/\//, "")}
        </a>{" "}
        as “{p.event === "APPROVE" ? "Approve" : p.event === "REQUEST_CHANGES" ? "Request changes" : "Comment"}”, at commit <Text mono>{p.commit.slice(0, 7)}</Text>.
      </Text>
      {p.body && <pre>{p.body}</pre>}
      {p.comments.map((c) => (
        <div key={c.id}>
          <Text as="div" mono dim>
            {c.path}:{c.startLine ? `${c.startLine}–${c.line}` : c.line}
            {c.side === "LEFT" ? " (removed lines)" : ""}
          </Text>
          <pre>{c.body}</pre>
        </div>
      ))}
      {!p.body && !p.comments.length && <Text as="div" dim>No words: just the verdict.</Text>}
      {!!p.loose && (
        <Text as="div" dim className={css.loose}>
          <Icon name="info" size={12} /> {p.loose === 1 ? "One comment is" : `${p.loose} comments are`} on lines outside the pull request's diff on GitHub, so {p.loose === 1 ? "it goes" : "they go"} in the review's body instead.
        </Text>
      )}
    </div>
  );
}
