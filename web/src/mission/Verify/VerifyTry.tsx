import type { BundleTry } from "../../api";
import { Inline } from "../../ui/Inline";
import { MissionVerifyCommand } from "./MissionVerifyCommand";

// VerifyTry is something to do to see it yourself: its command to copy,
// then the rest in words; or what to look for.
export function VerifyTry({ t }: { t: BundleTry }) {
  if (t.look) {
    return (
      <span>
        <span className="dim">Look for</span> <Inline text={t.text} />
      </span>
    );
  }
  if (!t.command) return <Inline text={t.text} />;
  const rest = t.text.replace(/^`[^`]+`\s*/, "");
  return (
    <>
      <MissionVerifyCommand text={t.command} />
      {rest && (
        <>
          {" "}
          <Inline text={rest} />
        </>
      )}
    </>
  );
}
