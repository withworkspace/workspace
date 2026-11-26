import { useState } from "react";
import { type ReviewGuide, runPRGuide } from "../api";
import { Banner } from "../ui/Banner";
import { Button } from "../ui/Button";
import { humanError } from "../ui/strings";
import { toast } from "../ui/toast";
import { GuideBox } from "./GuideBox";
import { GuideFoot } from "./GuideFoot";
import { GuideText } from "./GuideText";
import { GuideWriting } from "./GuideWriting";
import { PassNote } from "./PassNote";
import { Text } from "../ui/Text";

// A review's guide, a view of its own: what the change does, where that
// differs from what it says it does, and the order to read its files in.
// It hides nothing: the list of files can follow its order, or their
// folders'. GuideSection writes it (again), and shows it as far as it's
// got while it's written.
export function GuideSection({ api, guide, guided, onGuide, onGuided }: { api: string; guide?: ReviewGuide; guided: boolean; onGuide: (g: ReviewGuide) => void; onGuided: (on: boolean) => void }) {
  const [busy, setBusy] = useState(false);
  if (!guide) {
    return (
      <>
        <h2>Guide</h2>
        <Text as="p" dim>Loading guide…</Text>
      </>
    );
  }
  const run = () => {
    if (busy || guide.running) return;
    setBusy(true);
    runPRGuide(api)
      .then(onGuide, (e: Error) => toast(humanError(e.message), { tone: "danger" }))
      .finally(() => setBusy(false));
  };
  const g = guide.guide;
  const running = !!guide.running || busy;
  return (
    <>
      <h2>Guide</h2>
      <GuideBox>
        {!g && !running && (
          <Text as="p" dim>
            What the change does, where it differs from its description, and the order to read its files.{" "}
            <Button size="small" onClick={run}>Write guide</Button>
          </Text>
        )}
        {running && <GuideWriting since={guide.since} thinking={guide.thinking} given={guide.given} again={!!g} />}
        {guide.failed && !running && <Banner>{humanError(guide.failed)}</Banner>}
        {running && guide.draft ? (
          <GuideText g={guide.draft} writing />
        ) : (
          g && (
            <>
              {guide.moved && !running && (
                <PassNote kind="moved">
                  {" "}
                  It's been pushed to since this was written{guide.ran ? ` (at ${guide.ran.slice(0, 7)})` : ""}: files it doesn't place come first.
                </PassNote>
              )}
              <GuideText g={g} />
            </>
          )
        )}
        {g && !(running && guide.draft) && <GuideFoot guided={guided} at={guide.at} running={running} onGuided={onGuided} onRun={run} />}
      </GuideBox>
    </>
  );
}
