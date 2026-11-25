import type { PassFinding } from "../api";
import { Markdown } from "../markdown/LazyMarkdown";
import { Chip } from "../ui/Chip";
import type { Tone } from "../ui/tone";
import { PageFindingHead } from "./PageFindingHead";
import { PageFindingQuote } from "./PageFindingQuote";
import { PageFindingSuggest } from "./PageFindingSuggest";
import { PageFindingText } from "./PageFindingText";

const kindTone: Record<string, Tone> = { requirement: "done", vague: "attention", assumption: "waiting", missing: "attention", ambiguity: "attention" };

// FindingBody is what a finding says: its kind, the words it's about (to
// go to, with onWords, unless they're lost from the page), what it has to
// say of them, and the words it proposes in their place.
export function FindingBody({ f, lost, onWords }: { f: PassFinding; lost?: boolean; onWords?: () => void }) {
  return (
    <>
      <PageFindingHead>
        <Chip tone={kindTone[f.kind] ?? "quiet"}>{f.kind}</Chip>
        {f.quote && (
          <PageFindingQuote lost={lost} onClick={onWords}>
            {f.quote}
          </PageFindingQuote>
        )}
      </PageFindingHead>
      <PageFindingText>
        <Markdown text={f.text} />
      </PageFindingText>
      {f.suggest && (
        <PageFindingSuggest>
          <Markdown text={f.suggest} />
        </PageFindingSuggest>
      )}
    </>
  );
}
