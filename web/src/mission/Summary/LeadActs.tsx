import { useState } from "react";
import { Link, useNavigate } from "react-router";
import { act, type ActVerb, type MissionAct, type MissionCard, type MissionDetail } from "../../api";
import { type Reload, useHere } from "../../load";
import { Button, buttonClass } from "../../ui/Button";
import { hintOnClick } from "../../ui/keyHint";
import { Icon, type IconName } from "../../ui/Icon";
import { Input } from "../../ui/Input";
import { Keys } from "../../ui/Keys";
import { humanError } from "../../ui/text";
import { toast } from "../../ui/toast";
import { archiveMission } from "../archive";
import { ended } from "../Ending/ended";
import { actHref } from "../hrefs";
import { LeadCrewLink } from "./LeadCrewLink";
import { LeadPrompt } from "./LeadPrompt";
import { LeadReply } from "./LeadReply";
import { MissionLeadAnswer, MissionLeadAnswerForm, MissionLeadButtons } from "./MissionLeadAnswer";
import { MissionLeadLaunch } from "./MissionLeadLaunch";
import { actsOf, actsShape } from "./words";

// LeadActs is what a card offers, in the order it's given: the one meant
// to move it first, accented, then the rest, the quiet ones quiet. An act
// that takes words has its field beside it when it's the card's first, and
// opens it when pressed otherwise. How each is done is here; which there
// are, and in what order, is ws's to say (mission.Cards).
export function LeadActs({ d, c, lead, reload }: { d: MissionDetail; c: MissionCard; lead: boolean; reload: Reload }) {
  const navigate = useNavigate();
  const here = useHere();
  const [busy, setBusy] = useState(false);
  const [words, setWords] = useState("");
  const [opened, setOpened] = useState<MissionAct | undefined>();
  const send = (verb: ActVerb, body: { text?: string; crew?: string; task?: number; harness?: string; model?: string } = {}) => {
    setBusy(true);
    // Archiving asks first when it would burn work that hasn't merged.
    (verb === "archive" ? archiveMission(d.slug) : act(d.slug, verb, body))
      .then((r) => {
        if (!r) return;
        if (verb === "end") return ended(d.slug, r.message, reload);
        // Where it takes you (briefing: its own page) is for the card you
        // pressed it on: gone somewhere else since, you're told instead.
        if (r.path && here()) return navigate(r.path);
        toast(r.message, {
          tone: "done",
          action:
            verb === "archive"
              ? {
                  label: "Undo",
                  run: () =>
                    act(d.slug, "unarchive").then(
                      (x) => {
                        toast(x.message);
                        reload();
                      },
                      (e: Error) => toast(humanError(e.message), { tone: "danger" }),
                    ),
                }
              : undefined,
        });
        return reload();
      })
      .catch((e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  // run does an act, with what was written in its field, if it has one.
  const run = (a: MissionAct, written = "") => {
    const said = written.trim();
    switch (a.do) {
      case "answer": {
        const text = said ? `${a.prefix ?? ""}${said}` : a.text;
        if (text) send("answer", { text, task: a.task || undefined });
        return;
      }
      case "accept":
        return send("accept", { text: said });
      case "resume":
        return send("resume", { crew: a.target });
      case "not-yet":
        return send("dismiss");
      default:
        return send(a.do as ActVerb);
    }
  };
  const acts = actsOf(c, lead);
  const first = c.primary && acts[0] === c.primary ? c.primary : undefined;
  // Crew stopped on something are answered where they are: what they're
  // asking, here; a reply, written here.
  const shape = actsShape(d, c, lead);
  if (shape !== "acts") {
    const crew = d.crew.find((x) => x.id === first?.target && !x.ended);
    if (!crew || !first) return null;
    if (shape === "prompt") return <LeadPrompt slug={d.slug} crew={crew} />;
    if (shape === "reply") return <LeadReply slug={d.slug} id={crew.id} reload={reload} />;
    return <LeadCrewLink slug={d.slug} id={crew.id} verb={first.do} />;
  }
  const one = (a: MissionAct, i: number) => {
    const accent = a === first;
    // The first act is the one accented, where the card leads the page: a
    // notice's first reads as an ordinary button.
    const variant = accent ? (lead ? "primary" : undefined) : a.quiet ? "ghost" : undefined;
    const inside = (
      <>
        {a.icon && <Icon name={a.icon as IconName} />} {a.label} {lead && a.key && <Keys k={a.key} />}
      </>
    );
    // Its key is the page's (v, for the lead's review), which the lead's
    // shows in it: pressed with the pointer, it hints at it.
    const keys = lead ? a.key || undefined : undefined;
    if (a.do === "launch") return <MissionLeadLaunch key={i} picks={d.crewPicks} primary={accent && lead} again={!accent} busy={busy} launch={(p) => send("launch", { harness: p?.harness, model: p?.model })} />;
    if (a.do === "url") {
      // GitHub hasn't said where it is yet: its words, with nothing to open.
      if (!a.url) {
        return (
          <span key={i} className="dim">
            {a.label}
          </span>
        );
      }
      return (
        <a key={i} href={a.url} target="_blank" rel="noopener" className={buttonClass({ variant: accent && lead ? "primary" : undefined })} onClick={hintOnClick(keys)}>
          {inside} <Icon name="external" size={12} />
        </a>
      );
    }
    const href = actHref(d, c, a);
    if (href) {
      return (
        <Link key={i} to={href} className={buttonClass({ variant })} onClick={hintOnClick(keys)}>
          {inside}
        </Link>
      );
    }
    // One that takes words opens its field, but the card's first, whose
    // field is beside it.
    if (a.field && !accent) {
      return opened === a ? null : (
        <Button
          key={i}
          variant={variant}
          disabled={busy}
          keys={keys}
          onClick={() => {
            setWords("");
            setOpened(a);
          }}
        >
          {inside}
        </Button>
      );
    }
    return (
      <Button key={i} variant={variant} type={accent && a.field ? "submit" : "button"} disabled={busy || (accent && !!a.field && !a.text && a.do === "answer" && !words.trim() && !opened)} keys={keys} onClick={accent && a.field ? undefined : () => run(a)}>
        {inside}
      </Button>
    );
  };
  // Words are all the first asks for: the field, then its button.
  const wordsOnly = !!first?.field && first.do === "answer" && !first.text;
  const field = (a: MissionAct, focus: boolean) => (
    <Input value={words} onChange={(e) => setWords(e.target.value)} placeholder={a.field} disabled={busy} aria-label={a.field} autoFocus={focus} />
  );
  return (
    <MissionLeadAnswer>
      {first?.field && !opened ? (
        <>
          <MissionLeadAnswerForm onSubmit={() => run(first, words)}>
            {wordsOnly && field(first, false)}
            {one(first, 0)}
            {!wordsOnly && field(first, false)}
          </MissionLeadAnswerForm>
          {acts.length > 1 && <MissionLeadButtons>{acts.slice(1).map((a, i) => one(a, i + 1))}</MissionLeadButtons>}
        </>
      ) : (
        <MissionLeadButtons>{acts.map((a, i) => (a === first && a.field ? <Button variant="primary" key={i} disabled={busy} onClick={() => setOpened(undefined)}>{a.label}…</Button> : one(a, i)))}</MissionLeadButtons>
      )}
      {opened && (
        <MissionLeadAnswerForm onSubmit={() => run(opened, words)}>
          {field(opened, true)}
          <Button type="submit" disabled={busy || (opened.do === "answer" && !opened.text && !words.trim())}>
            {opened.label.replace(/…$/, "")}
          </Button>
        </MissionLeadAnswerForm>
      )}
    </MissionLeadAnswer>
  );
}
