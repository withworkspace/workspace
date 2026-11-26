import { useEffect, useRef, useState } from "react";
import { getReview, noteKinds, type Posting, type PR, type PRDetail, prAPI, previewPRReview, type Review as ReviewData, type ReviewPass, type ReviewState, submitPRReview, type Verdict } from "../api";
import { useKeys } from "../keys";
import { ReviewVerdicts } from "../review/ReviewVerdicts";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Callout } from "../ui/Callout";
import { Icon } from "../ui/Icon";
import { Keys } from "../ui/Keys";
import { Spinner } from "../ui/Spinner";
import { humanError } from "../ui/strings";
import { TextArea } from "../ui/TextArea";
import { standingWords } from "./passWords";
import { PullRequestPosting } from "./PullRequestPosting";
import { PullRequestSubmit } from "./PullRequestSubmit";
import { pullRequestLeftAloneClass, PullRequestSubmitLayer } from "./PullRequestSubmitLayer";
import { Text } from "../ui/Text";

// Posted is a review that's gone to GitHub: the pull request it's of, and
// where it is there.
export interface Posted {
  number: number;
  url?: string;
  // It's on its way to GitHub, posted behind: not there yet.
  sending?: boolean;
}

interface LayerDraft {
  pr: PR;
  draft: ReviewData | null;
  verdict?: Verdict;
  summary: string;
  posting?: Posting;
  error?: string;
  state?: "posting" | "posted";
}

// PRSubmitDialog ends the review by posting it to GitHub, as you: the one
// thing here that leaves your machine. It shows what will be posted, and
// where, before it is. GitHub reviews one pull request at a time, so a
// stack's lists its layers, each with its comments and its own verdict; a
// layer with no verdict is left alone.
export function PRSubmitDialog({
  d,
  passes,
  head,
  onClose,
  onDiscard,
  onPosted,
}: {
  d: PRDetail;
  passes?: ReviewPass[]; // this pull request's, as they stand
  head: string;
  onClose: () => void;
  onDiscard: () => void;
  // Every review chosen is on GitHub: the layers posted while this was open,
  // and this pull request's reviews as they then are, if it was one of them.
  onPosted: (posted: Posted[], mine?: ReviewState) => void;
}) {
  const prs = d.stack?.layers ?? [d.pr];
  const [layers, setLayers] = useState<LayerDraft[]>(() => prs.map((pr) => ({ pr, draft: null, summary: "" })));
  const [focus, setFocus] = useState(() => Math.max(0, prs.findIndex((p) => p.number === d.pr.number)));
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const area = useRef<HTMLTextAreaElement>(null);
  // What's been posted while this is open, a failed try before it included.
  const posted = useRef<Posted[]>([]);
  const mine = useRef<ReviewState | undefined>(undefined);
  const set = (i: number, patch: Partial<LayerDraft>) => setLayers((ls) => ls.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  // The commit a layer's review is of: the one this page shows, for the
  // layer it's on; for another, the one its draft was written on.
  const headOf = (l: LayerDraft) => (l.pr.number === d.pr.number ? head : (l.draft?.head ?? ""));

  // Each layer's draft, as it's kept.
  useEffect(() => {
    let stopped = false;
    Promise.all(prs.map((pr) => getReview(prAPI(pr.repo, pr.number)).then((s) => s.draft, () => null))).then((drafts) => {
      if (stopped) return;
      // (A review GitHub didn't take, made a draft again, has what was
      // written with it.)
      setLayers((ls) => ls.map((l, i) => ({ ...l, draft: drafts[i], summary: l.summary || drafts[i]?.summary || "" })));
      setLoaded(true);
    });
    return () => {
      stopped = true;
    };
  }, []);

  // What a layer's verdict and summary would post, asked for as they change.
  const want = layers.map((l) => `${l.verdict ?? ""}\u0000${l.summary}\u0000${l.draft?.comments.length ?? 0}`).join("\u0001");
  // (An answer that comes once they've changed again is dropped.)
  useEffect(() => {
    if (!loaded) return;
    let stale = false;
    const t = setTimeout(() => {
      layers.forEach((l, i) => {
        if (!l.verdict || l.state) return;
        previewPRReview(prAPI(l.pr.repo, l.pr.number), headOf(l), l.verdict, l.summary).then(
          (posting) => !stale && set(i, { posting, error: undefined }),
          (e: Error) => !stale && set(i, { posting: undefined, error: e.message }),
        );
      });
    }, 250);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  }, [want, loaded]);

  const pick = (i: number, v?: Verdict) => {
    if (layers[i].state) return;
    set(i, { verdict: layers[i].verdict === v ? undefined : v, posting: undefined, error: undefined });
    setFocus(i);
    setTimeout(() => area.current?.focus());
  };
  const approveAll = () => {
    setLayers((ls) => ls.map((l) => (l.state || l.pr.mine || l.pr.draft ? l : { ...l, verdict: "approve" as Verdict, posting: undefined, error: undefined })));
  };
  const chosen = layers.filter((l) => l.verdict && !l.state);
  const ready = chosen.length > 0 && chosen.every((l) => l.posting && !l.error);

  // Posted bottom first, one pull request at a time: a layer that fails
  // stops there, and those before it are on GitHub.
  const submit = async () => {
    if (!ready || busy) return;
    setBusy(true);
    let failed = false;
    for (let i = 0; i < layers.length; i++) {
      const l = layers[i];
      if (!l.verdict || l.state === "posted") continue;
      set(i, { state: "posting", error: undefined });
      try {
        const next = await submitPRReview(prAPI(l.pr.repo, l.pr.number), headOf(l), l.verdict, l.summary);
        set(i, { state: "posted" });
        posted.current.push({ number: l.pr.number, url: next.url, sending: next.sending });
        if (l.pr.number === d.pr.number) mine.current = next;
      } catch (e) {
        set(i, { state: undefined, error: (e as Error).message });
        failed = true;
        break;
      }
    }
    setBusy(false);
    if (!failed) onPosted(posted.current, mine.current);
  };

  useKeys(
    {
      a: () => !layers[focus].pr.mine && pick(focus, "approve"),
      c: () => !layers[focus].pr.mine && pick(focus, "changes"),
      m: () => pick(focus, "comment"),
      A: approveAll,
      "[": () => setFocus((f) => Math.max(0, f - 1)),
      "]": () => setFocus((f) => Math.min(layers.length - 1, f + 1)),
      "mod+Enter": submit,
    },
    { modal: true },
  );

  // Passes with a finding that blocks still open: approving over one is
  // said, not stopped.
  const noGo = (passes ?? []).filter((p) => p.standing.status === "nogo");
  const stack = layers.length > 1;
  const here = layers[Math.max(0, prs.findIndex((p) => p.number === d.pr.number))];
  return (
    <PullRequestSubmit
      title={stack ? "Submit stack review" : "Submit review"}
      onClose={onClose}
      intro={
        <>
          Posts to GitHub as you. A review cannot be unsent, only dismissed. Below is exactly what is posted.
          {stack && " Each layer gets its own verdict. Layers without one are skipped."}
        </>
      }
      all={
        stack && (
          <>
            <Button onClick={approveAll} disabled={busy} title="Approves every layer except drafts and your own. Nothing posts until you submit." keys="A">
              <Icon name="check-circle" size={13} /> Approve every layer <Keys k="A" />
            </Button>
            <Text dim>
              <Keys k="[" /> <Keys k="]" /> pick a layer; <Keys k="a" /> <Keys k="c" /> <Keys k="m" /> its verdict
            </Text>
          </>
        )
      }
      start={
        here?.draft &&
        !busy && (
          <Button variant="ghost" onClick={onDiscard} title={`Discard draft comments on #${d.pr.number}`}>
            Discard draft
          </Button>
        )
      }
      end={
        <>
          <Button variant="ghost" onClick={onClose} keys="esc">
            {layers.some((l) => l.state === "posted") ? "Close" : "Cancel"}
          </Button>
          <Button variant="primary" onClick={submit} disabled={busy} unavailable={ready ? undefined : "Pick a verdict first"} title="Post to GitHub" keys="mod+⏎">
            {busy ? "Posting…" : chosen.length > 1 ? `Post ${chosen.length} reviews to GitHub` : "Post to GitHub"} <Keys k="mod+⏎" />
          </Button>
        </>
      }
    >
      {layers.map((l, i) => {
        const counts = noteKinds.flatMap((k) => {
          const n = l.draft?.comments.filter((c) => c.kind === k).length ?? 0;
          return n ? [`${n} ${k}`] : [];
        });
        const total = l.draft?.comments.length ?? 0;
        return (
          <PullRequestSubmitLayer
            key={l.pr.number}
            n={stack ? i + 1 : undefined}
            title={l.pr.title}
            at={
              <>
                {l.pr.repo}#{l.pr.number}
              </>
            }
            said={total ? `${total} ${total === 1 ? "comment" : "comments"}${counts.length ? ` (${counts.join(", ")})` : ""}` : "no comments"}
            state={
              l.state === "posted" ? (
                <span className="settled-line done">
                  <Icon name="check-circle" size={13} /> Posted
                </span>
              ) : l.state === "posting" ? (
                <Text dim>
                  <Spinner size={12} /> Posting…
                </Text>
              ) : undefined
            }
            focus={i === focus}
            posted={l.state === "posted"}
            onFocus={() => setFocus(i)}
          >
            {!l.state && (
              <ReviewVerdicts
                label={`Verdict for #${l.pr.number}`}
                value={l.verdict}
                keyed={i === focus}
                unavailable={(v) => (l.pr.mine && v !== "comment" ? "Your own pull request: comment only" : undefined)}
                onPick={(v) => pick(i, v)}
                after={stack && !l.verdict && <Text dim className={pullRequestLeftAloneClass}>Skipped</Text>}
              />
            )}
            {l.verdict && !l.state && (
              <TextArea
                ref={i === focus ? area : undefined}
                value={l.summary}
                rows={2}
                placeholder="Summary (optional)"
                aria-label={`Summary for #${l.pr.number}`}
                onFocus={() => setFocus(i)}
                onChange={(e) => set(i, { summary: e.target.value })}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                    e.preventDefault();
                    submit();
                  }
                }}
              />
            )}
            {l.verdict === "approve" && !l.state && l.pr.number === d.pr.number && noGo.length > 0 && (
              <Callout tone="attention" icon="alert" title={noGo.length === 1 ? `${noGo[0].title} is no go` : `${noGo.length} passes are no go`}>
                {noGo.map((p) => `${p.title}: ${standingWords(p.standing).toLowerCase()}`).join("; ")}. You
                can still approve: add valid findings to your review, or dismiss the rest.
              </Callout>
            )}
            {l.error && <Banner>{humanError(l.error)}</Banner>}
            {l.verdict && l.posting && !l.state && <PullRequestPosting p={l.posting} />}
          </PullRequestSubmitLayer>
        );
      })}
    </PullRequestSubmit>
  );
}
