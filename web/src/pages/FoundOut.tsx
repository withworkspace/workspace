import { useState } from "react";
import type { PageCell } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Button } from "../ui/Button";
import { Icon } from "../ui/Icon";
import { SendKeys } from "../ui/SendKeys";
import { sendKeys, useSendKey } from "../ui/sendKey";
import { Spinner } from "../ui/Spinner";
import { Turn, TurnSaid } from "../ui/Turn";
import { askingNow, liveCalls, versions } from "./cell";
import { PageCalls } from "./PageCalls";
import { PageComposer } from "./PageComposer";
import { PageFoundOut } from "./PageFoundOut";
import { PageTurn } from "./PageTurn";
import { PageTurns } from "./PageTurns";
import { PageVersion } from "./PageVersion";
import { Text } from "../ui/Text";

// FoundOut is what an agent found out about a finding, under it on the
// pass's tab: its answers, what you asked after, and a box to ask more, or
// to have it written into the page. Until then nothing of it is in the
// page.
export function FoundOut({ cell, root, onAsk, onPut, onCell }: { cell: PageCell; root: string; onAsk: (body: string) => Promise<void>; onPut: (body: string) => void; onCell: () => void }) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const sendKey = useSendKey();
  const written = versions(cell).length > 0;
  const last = cell.turns.at(-1);
  const ask = () => {
    const body = text.trim();
    if (!body || busy) return;
    setBusy(true);
    onAsk(body)
      .then(() => setText(""))
      .finally(() => setBusy(false));
  };
  return (
    <PageFoundOut>
      <PageTurns flush>
        {cell.turns.slice(1).map((t) =>
          t.by === "you" ? (
            <PageTurn key={t.n}>
              <Turn by="you" dense landed>
                <TurnSaid written>{t.body}</TurnSaid>
              </Turn>
            </PageTurn>
          ) : t.by !== "agent" ? (
            <PageTurn key={t.n} kind="failed">
              <Icon name="alert" size={12} /> {t.body}
            </PageTurn>
          ) : t.answer ? (
            <PageTurn key={t.n} kind="answer">
              <Turn by="agent" dense landed>
                <TurnSaid>
                  <Markdown text={t.body} pinned={t.refs} />
                </TurnSaid>
                <PageCalls calls={t.calls ?? []} root={root} />
              </Turn>
            </PageTurn>
          ) : (
            <PageTurn key={t.n} kind="version">
              <PageVersion>
                <Icon name="sparkle" size={12} />
                <span>Written into the page</span>
                <Button variant="link" onClick={onCell}>
                  Go to it
                </Button>
              </PageVersion>
            </PageTurn>
          ),
        )}
        {cell.answering && (
          <PageTurn kind="filling">
            <PageVersion>
              <Spinner size={12} />
              <span>{askingNow(cell) ? "Finding out" : "Writing it into the page"}</span>
            </PageVersion>
            <PageCalls calls={liveCalls(cell.run)} root={root} open live />
          </PageTurn>
        )}
        {!cell.answering && last?.by === "you" && (
          <PageTurn kind="failed">
            <Text dim>Its agent hasn't started.</Text>
          </PageTurn>
        )}
      </PageTurns>
      {!cell.answering && last?.by !== "you" && (
        <PageComposer
          flush
          value={text}
          onChange={setText}
          onSend={ask}
          tallest={120}
          placeholder={written ? "Ask more…" : "Ask more, or say what to put in the page…"}
          aria-label="Ask more about this finding"
          escape="leave"
        >
          {!written && (
            <Button size="small" disabled={busy} onClick={() => onPut(text.trim())} title="Have it written into the page, under the words this is about: what it's answered, or what you say">
              Put it in the page
            </Button>
          )}
          <Button variant="primary" size="small" disabled={!text.trim() || busy} onClick={ask} title="Ask: it's answered here, and the page stays as it is" keys={sendKeys(sendKey)}>
            Ask <SendKeys />
          </Button>
        </PageComposer>
      )}
    </PageFoundOut>
  );
}
