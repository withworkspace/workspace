import type { Node as PMNode } from "prosemirror-model";
import { useMemo, useState } from "react";
import { type DismissWhy, dismissWhys, type PageCell, type PagePass, type PassFinding } from "../api";
import { onPage } from "../editor/notes";
import { Markdown } from "../markdown/LazyMarkdown";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { Calls } from "../ui/Calls";
import { Icon } from "../ui/Icon";
import { MoreLink } from "../ui/MoreLink";
import { Spinner } from "../ui/Spinner";
import { Time } from "../ui/Time";
import { liveCalls } from "./cell";
import { FindingBody } from "./FindingBody";
import { FoundOut } from "./FoundOut";
import { PageDismissAs } from "./PageDismissAs";
import { PageFinding } from "./PageFinding";
import { PageFindingActs } from "./PageFindingActs";
import { PageFindings } from "./PageFindings";
import { PagePassHead } from "./PagePassHead";
import { PagePassMeta } from "./PagePassMeta";
import { PagePassPanel } from "./PagePassPanel";
import { PagePassRunning } from "./PagePassRunning";
import { PagePassShown } from "./PagePassShown";
import { PagePassSummary } from "./PagePassSummary";

// PassTab is a pass over the page, on its tab: what it is, when it ran and
// what it said, and what it found, each to deal with. Run again, it reads
// the page as it is now; what you've settled stays settled.
export function PassTab({
  pass,
  root,
  doc,
  rev,
  shown,
  onShown,
  onRun,
  onSettle,
  onWords,
  onTake,
  onFindOut,
  asked,
  onAsk,
  onPut,
  onCell,
}: {
  pass: PagePass;
  root: string;
  doc?: PMNode;
  rev: number;
  shown: boolean;
  onShown: (on: boolean) => void;
  onRun: () => Promise<void>;
  onSettle: (f: PassFinding, state: PassFinding["state"], why?: DismissWhy) => void;
  onWords: (f: PassFinding) => void;
  onTake: (f: PassFinding) => void;
  onFindOut: (f: PassFinding) => void;
  // asked is the question asked of a finding, if one has been; onAsk asks
  // more in it, onPut has its answer written into the page, and onCell
  // goes to the cell it's become.
  asked: (f: PassFinding) => PageCell | undefined;
  onAsk: (cell: PageCell, body: string) => Promise<void>;
  onPut: (f: PassFinding, cell: PageCell, body: string) => void;
  onCell: (cell: PageCell) => void;
}) {
  const [starting, setStarting] = useState(false);
  const [settled, setSettled] = useState(false);
  const start = () => {
    setStarting(true);
    onRun().finally(() => setStarting(false));
  };
  const open = pass.findings.filter((f) => f.state === "open");
  const put = pass.findings.filter((f) => f.state !== "open");
  // Whose words have gone from the page since.
  const lost = useMemo(() => new Set(doc ? pass.findings.filter((f) => f.quote && !onPage(doc, f.quote)).map((f) => f.key) : []), [doc, pass.findings, rev]);
  const busy = starting || !!pass.running;
  return (
    <PagePassPanel label={pass.title}>
      <PagePassHead
        about={pass.about}
        how={
          <>
            {pass.skill ? (
              <>
                Its agent follows the workspace's skill <span className="mono">{pass.skill}</span>
                {pass.args ? <>, asked “{pass.args}”</> : null}, and reads what it needs of the workspace.{" "}
              </>
            ) : pass.cold ? (
              "Its agent is given the page and nothing else. "
            ) : (
              "Its agent reads the page and what it needs of the workspace. "
            )}
            What it says is kept here, beside the page: none of it is written into the page.
          </>
        }
        unavailable={pass.unavailable}
        action={
          <Button variant={pass.runs ? undefined : "primary"} disabled={busy || !!pass.unavailable} onClick={start}>
            {busy ? (
              <>
                <Spinner size={12} /> Running…
              </>
            ) : pass.runs ? (
              <>
                <Icon name="rotate" size={12} /> Run again
              </>
            ) : (
              <>
                <Icon name="play" size={12} /> Run
              </>
            )}
          </Button>
        }
      />
      {pass.failed && !pass.running && <Banner>The last run couldn't finish. {pass.failed}</Banner>}
      {pass.moved && !pass.running && !!pass.runs && (
        <Banner
          tone="attention"
          actions={
            <Button size="small" disabled={busy} onClick={start}>
              Run again
            </Button>
          }
        >
          The page has changed since this ran: what's below is of the page as it was.
        </Banner>
      )}
      {pass.running && (
        <PagePassRunning words={pass.runs ? "Reading the page again" : "Reading the page"}>
          <Calls calls={liveCalls(pass.run)} root={root} open live />
        </PagePassRunning>
      )}
      {!!pass.runs && (
        <>
          <PagePassMeta
            at={
              pass.at && (
                <>
                  Ran <Time iso={pass.at} as="clock" />
                  {pass.runs > 1 ? `, ${pass.runs} times in all` : ""}
                </>
              )
            }
          >
            <PagePassShown shown={shown} onShown={onShown} />
          </PagePassMeta>
          {pass.summary && (
            <PagePassSummary>
              <Markdown text={pass.summary} />
            </PagePassSummary>
          )}
          {open.length === 0 && put.length === 0 && <p className="dim">It found nothing to raise.</p>}
          <PageFindings>
            {open.map((f) => {
              const q = asked(f);
              return (
                <PageFinding key={f.key}>
                  <FindingBody f={f} lost={lost.has(f.key)} onWords={() => onWords(f)} />
                  <PageFindingActs>
                    {f.suggest && f.quote && !lost.has(f.key) && (
                      <Button variant="primary" size="small" onClick={() => onTake(f)} title="Put these words on the page in place of those quoted">
                        Use these words
                      </Button>
                    )}
                    <Button size="small" onClick={() => onSettle(f, "done")} title="It's answered, or dealt with">
                      <Icon name="check" size={12} /> Done
                    </Button>
                    {!q && (
                      <Button variant="ghost" size="small" onClick={() => onFindOut(f)} title="Ask an agent to find this out: it answers here, under the finding, and nothing's put in the page">
                        Find out
                      </Button>
                    )}
                    <PageDismissAs whys={dismissWhys} onDismiss={(why) => onSettle(f, "dismissed", why)} />
                    {lost.has(f.key) && <span className="dim">Its words aren't on the page any more.</span>}
                  </PageFindingActs>
                  {q && <FoundOut cell={q} root={root} onAsk={(body) => onAsk(q, body)} onPut={(body) => onPut(f, q, body)} onCell={() => onCell(q)} />}
                </PageFinding>
              );
            })}
          </PageFindings>
          {put.length > 0 && (
            <>
              <MoreLink onClick={() => setSettled((v) => !v)} aria-expanded={settled}>
                {settled ? "Hide" : "Show"} {put.length} settled
              </MoreLink>
              {settled && (
                <PageFindings settled>
                  {put.map((f) => (
                    <PageFinding key={f.key} look="settled">
                      <FindingBody f={f} />
                      <PageFindingActs>
                        <span className="dim">{f.state === "done" ? "Done" : `Dismissed${f.why ? `: ${dismissWhys.find(([w]) => w === f.why)?.[1].toLowerCase()}` : ""}`}</span>
                        <Button variant="ghost" size="small" onClick={() => onSettle(f, "open")}>
                          Open it again
                        </Button>
                      </PageFindingActs>
                    </PageFinding>
                  ))}
                </PageFindings>
              )}
            </>
          )}
        </>
      )}
    </PagePassPanel>
  );
}
