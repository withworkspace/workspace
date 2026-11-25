import { Fragment, useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import { type Capsule, commentPR, type Discussion, getMission, getPR, type Mission, type PR, type Said } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Banner } from "../ui/Banner";
import { Icon } from "../ui/Icon";
import { useAutosize } from "../ui/useAutosize";
import { humanError } from "../ui/text";
import { Keys } from "../ui/Keys";
import { Time } from "../ui/Time";
import { Button } from "../ui/Button";
import { TextArea } from "../ui/TextArea";

// A review's overview and its discussion, views beside its changes. A pull
// request's overview is its description, as its author wrote it; a
// capsule's is its mission. The discussion is what's been said on the pull
// request itself on GitHub (reviews and comments; comments on lines are in
// the changes).

const saidWords: Record<Said["kind"], string> = {
  comment: "commented",
  approved: "approved",
  changes: "asked for changes",
  commented: "reviewed",
  dismissed: "had a review dismissed",
};

const saidIcon = (k: Said["kind"]) => (k === "approved" ? "check-circle" : k === "changes" ? "alert" : "message");

// PRDescription is a pull request's overview: what it says it does.
export function PRDescription({ body }: { body?: string }) {
  return (
    <>
      <h2>What it says it does</h2>
      {body?.trim() ? (
        <div className="prose">
          <Markdown text={body} />
        </div>
      ) : (
        <p className="dim">It has no description.</p>
      )}
    </>
  );
}

// MissionContext is a capsule's overview: the mission it's for, where that
// stands, and its brief, which says what the capsule's work is for.
export function MissionContext({ mission, capsule }: { mission: Mission; capsule: Capsule }) {
  const [brief, setBrief] = useState<string | null>();
  useEffect(() => {
    let stopped = false;
    getMission(mission.slug).then(
      (d) => !stopped && setBrief(d.brief),
      () => !stopped && setBrief(null),
    );
    return () => {
      stopped = true;
    };
  }, [mission.slug]);
  const h = mission.headline;
  return (
    <>
      <h2>Mission</h2>
      <p className="mission-context">
        <Link to={`/missions/${mission.slug}`} className="link">
          <Icon name="flag" size={12} /> {mission.title}
        </Link>{" "}
        <span className="dim">
          · <span className="mono">{capsule.repo}/{capsule.name}</span>
          {capsule.branch && (
            <>
              {" "}
              on <span className="mono">{capsule.branch}</span>
            </>
          )}
        </span>
      </p>
      {h.text && <p className="dim">{h.text}</p>}
      <h2>Brief</h2>
      {brief === undefined ? (
        <p className="dim">Reading the brief…</p>
      ) : brief === null ? (
        <p className="dim">The mission's brief couldn't be read.</p>
      ) : brief.trim() ? (
        <div className="prose">
          <Markdown text={brief} />
        </div>
      ) : (
        <p className="dim">The mission has no brief yet.</p>
      )}
    </>
  );
}

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
          <div className="said-list">
            {discussion.items.map((it) => (
              <SaidView key={it.id} it={it} />
            ))}
          </div>
          <SayBox api={api} onSaid={onSaid} />
        </>
      )}
    </>
  );
}

function SaidHead({ it }: { it: Said }) {
  return (
    <>
      <strong>{it.mine ? "You" : it.by || "Someone"}</strong>
      <span className="said-what">
        <Icon name={saidIcon(it.kind)} size={12} /> {saidWords[it.kind]}
      </span>
      {it.stale && (
        <span className="dim" title="It's had commits pushed since this review">
          on earlier commits
        </span>
      )}
      {it.sending ? (
        <span className="dim" title="It's on its way to GitHub">
          Sending…
        </span>
      ) : (
        <Time iso={it.at} as="relative" className="dim" />
      )}
      {it.url && (
        <a className="link dim" href={it.url} target="_blank" rel="noreferrer" title="See it on GitHub" onClick={(e) => e.stopPropagation()}>
          <Icon name="external" size={11} />
        </a>
      )}
    </>
  );
}

// SaidView is one thing said. A bot's is folded: it's rarely what you
// came to read.
function SaidView({ it }: { it: Said }) {
  if (it.bot) {
    return (
      <details className={`said k-${it.kind}`}>
        <summary>
          <SaidHead it={it} />
        </summary>
        {it.body && (
          <div className="prose">
            <Markdown text={it.body} />
          </div>
        )}
      </details>
    );
  }
  return (
    <article className={`said k-${it.kind}`}>
      <header>
        <SaidHead it={it} />
      </header>
      {it.body && (
        <div className="prose">
          <Markdown text={it.body} />
        </div>
      )}
    </article>
  );
}

// SayBox is you commenting on the pull request itself. It posts at once,
// as a reply in a thread does, and says so.
function SayBox({ api, onSaid }: { api: string; onSaid: (d: Discussion) => void }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const area = useRef<HTMLTextAreaElement>(null);
  useAutosize(area, text, 240);
  const send = () => {
    if (!text.trim() || busy) return;
    setBusy(true);
    commentPR(api, text)
      .then(
        (d) => {
          setText("");
          setError(undefined);
          onSaid(d);
        },
        (e: Error) => setError(e.message),
      )
      .finally(() => setBusy(false));
  };
  return (
    <div className="said-reply" onClick={(e) => e.stopPropagation()}>
      <TextArea
        ref={area}
        value={text}
        rows={2}
        placeholder="Comment on the pull request…"
        aria-label="Comment on the pull request"
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            send();
          } else if (e.key === "Escape") {
            e.preventDefault();
            e.currentTarget.blur();
          }
        }}
      />
      {error && <Banner>{humanError(error)}</Banner>}
      <div className="note-actions">
        <span className="dim">Posts to GitHub at once, as you: it isn't part of your draft.</span>
        <span className="spacer-left" />
        <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={send} title="Post it to GitHub (⌘⏎)">
          {busy ? "Posting…" : "Comment on GitHub"} <Keys k="mod+⏎" />
        </Button>
      </div>
    </div>
  );
}

// StackDescriptions is a whole stack's overview: each layer's
// description, bottom first, which is the order to read them in.
export function StackDescriptions({ layers }: { layers: PR[] }) {
  const [bodies, setBodies] = useState<Record<number, string | null>>({});
  const key = layers.map((l) => `${l.repo}#${l.number}`).join(" ");
  useEffect(() => {
    let stopped = false;
    for (const l of layers) {
      getPR(l.repo, l.number).then(
        (d) => !stopped && setBodies((b) => ({ ...b, [l.number]: d.body ?? "" })),
        () => !stopped && setBodies((b) => ({ ...b, [l.number]: null })),
      );
    }
    return () => {
      stopped = true;
    };
  }, [key]);
  return (
    <>
      {layers.map((l, i) => {
        const body = bodies[l.number];
        return (
          <Fragment key={l.number}>
            <h2>
              {i + 1}. {l.title} <span className="mono">#{l.number}</span>
            </h2>
            {body === undefined ? (
              <p className="dim">Reading it…</p>
            ) : body === null ? (
              <p className="dim">Its description couldn't be read.</p>
            ) : body.trim() ? (
              <div className="prose">
                <Markdown text={body} />
              </div>
            ) : (
              <p className="dim">It has no description.</p>
            )}
          </Fragment>
        );
      })}
    </>
  );
}
