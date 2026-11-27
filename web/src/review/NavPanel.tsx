import { Keys } from "../ui/Keys";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { getBlob } from "../api";
import { useCommand, useCommands } from "../keymap/useCommands";
import { reviewPanelKeys } from "./keys";
import { useSaid } from "../keymap/keymap";
import { Banner } from "../ui/Banner";
import { buttonClass } from "../ui/Button";
import { CodeLines } from "../ui/CodeLines";
import { keyProps } from "../ui/keyHint";
import { CodeNavPanel } from "./CodeNavPanel";
import { highlightLines, languageOf } from "./highlight";
import type { Side } from "./nav";
import { Text } from "../ui/Text";

// NavPanel is a file at a commit, beside the diff, at a definition: read,
// coloured and brought to the definition's line (ui/CodeLines). Names in
// it are links too, and following one stays in the panel. Its keys are
// its own while it's open: j, k and the half pages scroll it, o opens it
// in Files.
export function NavPanel({ api, at, browse, sideWords, onClose, onBack }: { api: string; at: { rev: string; path: string; line: number; side: Side; how?: string }; browse?: string; sideWords: (side: Side) => string; onClose: () => void; onBack: () => void }) {
  const navigate = useNavigate();
  const [lines, setLines] = useState<string[]>();
  const [error, setError] = useState<string>();
  useEffect(() => {
    getBlob(api, at.rev, at.path).then(setLines, (e: Error) => setError(e.message));
  }, [api, at.rev, at.path]);
  const body = useRef<HTMLDivElement>(null);
  const scroll = (by: number) => body.current?.scrollBy({ top: by * 20 });
  // Closing it and going back are a key and a button each: the key's
  // bound here, and the button's given what comes back.
  const close = useCommand(reviewPanelKeys.close, onClose, { modal: true });
  const goBack = useCommand(reviewPanelKeys.back, onBack, { modal: true });
  useCommands(
    reviewPanelKeys,
    {
      down: () => scroll(1),
      up: () => scroll(-1),
      halfDown: () => scroll(20),
      halfUp: () => scroll(-20),
      open: () => browse && navigate(browse),
    },
    { modal: true },
  );
  const openKey = useSaid(reviewPanelKeys.open);
  const html = useMemo(() => (lines ? highlightLines(lines, languageOf(at.path)) : []), [lines, at.path]);
  return (
    <CodeNavPanel
      label={`${at.path} at ${at.rev.slice(0, 7)}`}
      title={
        <Text mono>
          {at.path}:{at.line}
        </Text>
      }
      said={
        <>
          {sideWords(at.side)} · {at.rev.slice(0, 7)}
        </>
      }
      how={at.how}
      tools={
        browse && (
          <Link to={browse} className={buttonClass({ size: "small" })} {...keyProps(openKey, "Open current version in Files")}>
            Open <Keys k={reviewPanelKeys.open} />
          </Link>
        )
      }
      body={body}
      note={!lines && !error ? `Loading ${at.path}…` : undefined}
      onBack={goBack}
      onClose={close}
    >
      {error && <Banner>{error}</Banner>}
      {/* The file's lines (ui/CodeLines), gone to the definition's: brought to the middle and flashed. */}
      {lines && <CodeLines key={`${at.rev}:${at.path}`} html={html} path={at.path} rev={at.rev} side={at.side} reveal={at.line} />}
    </CodeNavPanel>
  );
}
