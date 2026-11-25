import type { PassMark } from "../api";
import { PassStanding } from "./PassStanding";
import { standingWords } from "./passWords";

// RowPasses are how the passes run over a pull request stand, on its row
// in the Reviews list: what you know before you open it.
export function RowPasses({ marks }: { marks?: PassMark[] }) {
  if (!marks?.length) return null;
  return (
    <>
      {marks.map((m) => (
        <PassStanding
          key={m.name}
          standing={m.standing}
          spin={m.standing.status === "running"}
          moved={m.moved}
          size={11}
          title={`The ${m.title} pass: ${standingWords(m.standing).toLowerCase()}${m.moved ? ". It's been pushed to since it ran" : ""}`}
        >
          {m.title}: {standingWords(m.standing).toLowerCase()}
        </PassStanding>
      ))}
    </>
  );
}
