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
import { Text } from "../ui/Text";

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
                Agent follows the workspace skill <Text mono>{pass.skill}</Text>
                {pass.args ? <>, asked “{pass.args}”</> : null}, and reads the workspace as needed.{" "}
              </>
            ) : pass.cold ? (
              "Agent sees only the page. "
            ) : (
              "Agent reads the page and the workspace as needed. "
            )}
            Results stay here; nothing is written into the page.
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
      {pass.failed && !pass.running && <Banner>Last run could not finish. {pass.failed}</Banner>}
      {pass.moved && !pass.running && !!pass.runs && (
        <Banner
          tone="attention"
          actions={
            <Button size="small" disabled={busy} onClick={start}>
              Run again
            </Button>
          }
        >
          Page changed since this ran: results below are for the earlier version.
        </Banner>
      )}
      {pass.running && (
        <PagePassRunning words={pass.runs ? "Rereading page" : "Reading page"}>
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
          {open.length === 0 && put.length === 0 && <Text as="p" dim>Nothing found.</Text>}
          <PageFindings>
            {open.map((f) => {
              const q = asked(f);
              return (
                <PageFinding key={f.key}>
                  <FindingBody f={f} lost={lost.has(f.key)} onWords={() => onWords(f)} />
                  <PageFindingActs>
                    {f.suggest && f.quote && !lost.has(f.key) && (
                      <Button variant="primary" size="small" onClick={() => onTake(f)} title="Replace quoted text with these words">
                        Use these words
                      </Button>
                    )}
                    <Button size="small" onClick={() => onSettle(f, "done")} title="Mark as done">
                      <Icon name="check" size={12} /> Done
                    </Button>
                    {!q && (
                      <Button variant="ghost" size="small" onClick={() => onFindOut(f)} title="Ask an agent; answers here without changing the page">
                        Find out
                      </Button>
                    )}
                    <PageDismissAs whys={dismissWhys} onDismiss={(why) => onSettle(f, "dismissed", why)} />
                    {lost.has(f.key) && <Text dim>Text no longer on the page.</Text>}
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
                        <Text dim>{f.state === "done" ? "Done" : `Dismissed${f.why ? `: ${dismissWhys.find(([w]) => w === f.why)?.[1].toLowerCase()}` : ""}`}</Text>
                        <Button variant="ghost" size="small" onClick={() => onSettle(f, "open")}>
                          Reopen
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
