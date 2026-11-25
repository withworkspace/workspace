import { getArtifactCompare } from "../api";
import { useLoaded } from "../load";
import { ErrorCard } from "../ui/ErrorCard";
import { Loading } from "../ui/Loading";
import { ArtifactCompareDiffCount } from "./ArtifactCompareDiff";
import { ArtifactEmpty } from "./ArtifactEmpty";
import { FileDiff } from "./FileDiff";
import { type Pair, sideRef } from "./pair";

// SourceDiff is the left's files against the right's: each file that
// differs, as a review shows a file's diff, under its path. One that's
// its page alone has the one.
export function SourceDiff({ pair, split }: { pair: Pair; split: boolean }) {
  const a = sideRef(pair[0]);
  const b = sideRef(pair[1]);
  const { value, error, reload } = useLoaded(() => getArtifactCompare(a, b), [a, b]);
  if (error) return <ErrorCard error={error} retry={reload} title={`Couldn't diff ${a} and ${b}`} />;
  if (!value) return <Loading what="Diffing their sources…" />;
  if (!value.files.length) {
    return (
      <ArtifactEmpty icon="check" title="The same">
        {value.a} and {value.b} are the same, byte for byte.
      </ArtifactEmpty>
    );
  }
  // Named by its path where there's more than the page to tell apart.
  const named = value.files.length > 1 || value.files[0].file.path !== "index.html";
  return (
    <>
      {named && <ArtifactCompareDiffCount files={value.files.length} a={value.a} b={value.b} />}
      {value.files.map((f) => (
        <FileDiff key={`${a}:${b}:${f.file.path}`} file={f.file} lines={f.lines} a={value.a} b={value.b} split={split} named={named} />
      ))}
    </>
  );
}
