import type { BundleTry } from "../../api";
import { Inline } from "../../ui/Inline";
import { MissionVerifyCommand } from "./MissionVerifyCommand";
import { useTryRuns } from "./VerifyTryRuns";
import { VerifyTryStart } from "./VerifyTryStart";
import { Text } from "../../ui/Text";

// VerifyTry is something to do to see it yourself: a recipe of the
// bundle's capsule (ws run web) to start with a button, where the capsule
// is there and has it; otherwise its command to copy, then the rest in
// words; or what to look for.
export function VerifyTry({ t }: { t: BundleTry }) {
  const at = useTryRuns();
  if (t.look) {
    return (
      <span>
        <Text dim>Look for</Text> <Inline text={t.text} />
      </span>
    );
  }
  const c = at?.capsule;
  if (at && c?.exists && t.recipe && ((c.recipes ?? []).includes(t.recipe) || c.runs?.some((r) => (r.recipe ?? r.name) === t.recipe))) {
    return <VerifyTryStart recipe={t.recipe} at={{ ...at, capsule: c }} />;
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
