import type { Bundle } from "../../api";
import { Chip } from "../../ui/Chip";
import { Inline } from "../../ui/Inline";
import { Printed } from "../../ui/Printed";
import { capital } from "../../ui/strings";
import { type CheckRow, verdictTone, verdictWords } from "../verifyRows";
import { MissionVerifyAsks } from "./MissionVerifyAsks";
import { MissionVerifyCheck } from "./MissionVerifyCheck";
import { MissionVerifyJudged } from "./MissionVerifyJudged";
import { MissionVerifyKept } from "./MissionVerifyKept";
import { MissionVerifyLines } from "./MissionVerifyLines";
import { MissionVerifyYours } from "./MissionVerifyYours";
import { MissionVerifyYourself } from "./MissionVerifyYourself";
import { VerifyEvidence } from "./VerifyEvidence";
import { VerifyTry } from "./VerifyTry";

// VerifyCheck is one check of the open bundle: what it came to, what was
// said of it, what shows it, and (while the bundle's yours to settle,
// kept at the pane's foot as the check scrolls) what's yours of it:
// Approve check for one left for you, Looked at for one that's settled,
// and Not right.
export function VerifyCheck({
  b,
  r,
  open,
  busy,
  look,
  note,
  setNote,
}: {
  b: Bundle;
  r: CheckRow;
  open: boolean;
  busy: boolean;
  look: () => void;
  note?: string; // what you've said isn't right of it; undefined when you haven't
  setNote: (text?: string) => void;
}) {
  const c = r.check;
  return (
    <MissionVerifyCheck
      verdict={<Chip tone={verdictTone(r.verdict)}>{verdictWords[r.verdict]}</Chip>}
      meta={
        <>
          {r.n ? `Check ${r.n}, ` : ""}
          {r.ran ? "Run by Workspace" : r.verdict === "yours" ? `left for you by ${b.crew}` : `${b.crew}${b.own ? ", who did the work" : ", its verifier"}`}
        </>
      }
      title={<Inline text={r.title} />}
      said={c?.check && <Inline text={c.said} />}
      points={c?.points?.map((pt) => <Inline text={pt} />)}
      yours={open && <MissionVerifyYours crew={b.crew} left={r.left} looked={r.looked} busy={busy} onLook={look} note={note} onNote={c ? setNote : undefined} />}
    >
      {c?.judged && (
        <MissionVerifyJudged tone={verdictTone(c.judged.result)}>
          <strong>
            {capital(c.judged.by)} says {c.judged.result}
          </strong>
          {c.judged.evidence ? (
            <>
              : <Inline text={c.judged.evidence} />
            </>
          ) : (
            "."
          )}
        </MissionVerifyJudged>
      )}
      {r.ran?.evidence && <Printed>{r.ran.evidence}</Printed>}
      {r.ran?.kept && <MissionVerifyKept to={`/runs?show=${encodeURIComponent(r.ran.kept)}`} />}
      {c?.shows?.map((e) => (
        <VerifyEvidence key={e.name} e={e} />
      ))}
      {c?.lines?.length ? <MissionVerifyLines lines={c.lines} /> : null}
      {c?.try?.length ? <MissionVerifyYourself items={c.try.map((t) => <VerifyTry t={t} />)} /> : null}
      {c?.asks?.length ? <MissionVerifyAsks crew={b.crew} asks={c.asks} onAsk={open ? setNote : undefined} /> : null}
    </MissionVerifyCheck>
  );
}
